package security

import "testing"

func TestExtractShellCommandsPerl(t *testing.T) {
	code := `
system('rm -rf /tmp/junk');
exec("ls -la");
my $out = ` + "`whoami`" + `;
qx{cat /etc/passwd};
qx(echo hello);
`

	cmds := ExtractShellCommands(code, "perl")
	if len(cmds) < 5 {
		t.Errorf("perl: got %d commands, want >= 5: %v", len(cmds), cmds)
	}
}

func TestExtractShellCommandsPerlNegative(t *testing.T) {
	code := `my $x = 42; print $x;`

	cmds := ExtractShellCommands(code, "perl")
	if len(cmds) != 0 {
		t.Errorf("perl negative: got %d commands, want 0: %v", len(cmds), cmds)
	}
}

func TestExtractShellCommandsR(t *testing.T) {
	code := `
system("ls -la")
system2("grep", args = c("-r", "pattern", "."))
shell("echo hello")
`

	cmds := ExtractShellCommands(code, "r")
	if len(cmds) < 3 {
		t.Errorf("R: got %d commands, want >= 3: %v", len(cmds), cmds)
	}
}

func TestExtractShellCommandsRNegative(t *testing.T) {
	code := `x <- 1:10; print(mean(x))`

	cmds := ExtractShellCommands(code, "r")
	if len(cmds) != 0 {
		t.Errorf("R negative: got %d commands, want 0: %v", len(cmds), cmds)
	}
}

func TestExtractShellCommandsElixir(t *testing.T) {
	code := `
System.cmd("ls", ["-la"])
:os.cmd('whoami')
`

	cmds := ExtractShellCommands(code, "elixir")
	if len(cmds) < 2 {
		t.Errorf("elixir: got %d commands, want >= 2: %v", len(cmds), cmds)
	}
}

func TestExtractShellCommandsElixirNegative(t *testing.T) {
	code := `IO.puts "hello world"`

	cmds := ExtractShellCommands(code, "elixir")
	if len(cmds) != 0 {
		t.Errorf("elixir negative: got %d commands, want 0: %v", len(cmds), cmds)
	}
}
