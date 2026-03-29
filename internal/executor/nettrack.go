package executor

import (
	"regexp"
	"strconv"
	"strings"
)

const netMarker = "__CM_NET__"

// JSNetPreamble is prepended to JavaScript/TypeScript code to instrument
// network byte tracking via monkey-patching http/https modules.
// Each request emits a line: __CM_NET__:<bytes>:<url>
const JSNetPreamble = `
(function() {
  try {
    const http = require('http');
    const https = require('https');
    const wrap = (mod) => {
      const orig = mod.request;
      mod.request = function(opts, cb) {
        let url = typeof opts === 'string' ? opts : (opts.href || opts.hostname || '');
        let bytes = 0;
        const req = orig.call(mod, opts, function(res) {
          res.on('data', (chunk) => { bytes += chunk.length; });
          res.on('end', () => {
            process.stderr.write('` + netMarker + `' + ':' + bytes + ':' + url + '\n');
          });
          if (cb) cb(res);
        });
        return req;
      };
    };
    wrap(http);
    wrap(https);
  } catch(e) {}
  try {
    const origFetch = globalThis.fetch;
    if (origFetch) {
      globalThis.fetch = async function(input, init) {
        const url = typeof input === 'string' ? input : (input.url || '');
        const resp = await origFetch(input, init);
        const cloned = resp.clone();
        cloned.arrayBuffer().then(buf => {
          const bytes = buf.byteLength;
          process.stderr.write('` + netMarker + `' + ':' + bytes + ':' + url + '\n');
        }).catch(() => {});
        return resp;
      };
    }
  } catch(e) {}
})();
`

// NetStats holds parsed network byte tracking results.
type NetStats struct {
	TotalBytes int      `json:"totalBytes"`
	Requests   int      `json:"requests"`
	URLs       []string `json:"urls,omitempty"`
}

var netMarkerRe = regexp.MustCompile(`(?m)^` + netMarker + `:(\d+):(.*)$`)

// ParseNetMarkers extracts network stats from stderr output containing markers.
// Marker format: __CM_NET__:<bytes>:<url>
// Returns nil if no markers are found.
func ParseNetMarkers(stderr string) *NetStats {
	matches := netMarkerRe.FindAllStringSubmatch(stderr, -1)
	if len(matches) == 0 {
		return nil
	}

	stats := &NetStats{}

	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		stats.TotalBytes += n

		stats.Requests++
		if url := strings.TrimSpace(m[2]); url != "" {
			stats.URLs = append(stats.URLs, url)
		}
	}

	return stats
}

// CleanNetMarkers removes network tracking markers from stderr output.
func CleanNetMarkers(stderr string) string {
	lines := strings.Split(stderr, "\n")

	var clean []string

	for _, line := range lines {
		if !strings.HasPrefix(line, netMarker+":") {
			clean = append(clean, line)
		}
	}

	return strings.Join(clean, "\n")
}
