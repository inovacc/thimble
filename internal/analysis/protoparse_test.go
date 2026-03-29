package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testProtoFile = `syntax = "proto3";

package myapi.v1;

// Health check service.
service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
}

service UserService {
  rpc GetUser(GetUserRequest) returns (UserResponse);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc DeleteUser(DeleteUserRequest) returns (Empty);
}

message HealthCheckRequest {}
message HealthCheckResponse {
  enum ServingStatus {
    UNKNOWN = 0;
    SERVING = 1;
  }
  ServingStatus status = 1;
}

message GetUserRequest {
  string id = 1;
}

message UserResponse {
  string id = 1;
  string name = 2;
}

message ListUsersRequest {}
message ListUsersResponse {
  repeated UserResponse users = 1;
}

message DeleteUserRequest {
  string id = 1;
}

message Empty {}
`

func writeTempProtoFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "service.proto")
	if err := os.WriteFile(path, []byte(testProtoFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParseProtoFilePackage(t *testing.T) {
	path := writeTempProtoFile(t)

	result, err := ParseProtoFile(path)
	if err != nil {
		t.Fatalf("ParseProtoFile: %v", err)
	}

	if result.Package != "myapi.v1" {
		t.Errorf("Package = %q, want %q", result.Package, "myapi.v1")
	}

	if result.Language != LangProto {
		t.Errorf("Language = %q, want %q", result.Language, LangProto)
	}
}

func TestParseProtoFileServices(t *testing.T) {
	path := writeTempProtoFile(t)

	result, err := ParseProtoFile(path)
	if err != nil {
		t.Fatalf("ParseProtoFile: %v", err)
	}

	services := filterByKind(result.Symbols, KindInterface)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	names := make(map[string]bool)
	for _, s := range services {
		names[s.Name] = true
	}

	if !names["Health"] {
		t.Error("missing service: Health")
	}

	if !names["UserService"] {
		t.Error("missing service: UserService")
	}
}

func TestParseProtoFileRPCs(t *testing.T) {
	path := writeTempProtoFile(t)

	result, err := ParseProtoFile(path)
	if err != nil {
		t.Fatalf("ParseProtoFile: %v", err)
	}

	rpcs := filterByKind(result.Symbols, KindMethod)
	if len(rpcs) != 4 {
		t.Fatalf("expected 4 RPCs, got %d", len(rpcs))
	}

	// Check receiver assignment.
	for _, rpc := range rpcs {
		if rpc.Receiver == "" {
			t.Errorf("RPC %s should have a receiver (service name)", rpc.Name)
		}
	}

	// Check specific RPC.
	found := false

	for _, rpc := range rpcs {
		if rpc.Name == "GetUser" && rpc.Receiver == "UserService" {
			found = true

			if rpc.Signature != "rpc GetUser(GetUserRequest) returns (UserResponse)" {
				t.Errorf("GetUser signature = %q", rpc.Signature)
			}
		}
	}

	if !found {
		t.Error("missing RPC: UserService.GetUser")
	}
}

func TestParseProtoFileMessages(t *testing.T) {
	path := writeTempProtoFile(t)

	result, err := ParseProtoFile(path)
	if err != nil {
		t.Fatalf("ParseProtoFile: %v", err)
	}

	messages := filterByKind(result.Symbols, KindStruct)
	if len(messages) < 7 {
		t.Errorf("expected at least 7 messages, got %d", len(messages))
	}
}

func TestParseProtoFileEnums(t *testing.T) {
	path := writeTempProtoFile(t)

	result, err := ParseProtoFile(path)
	if err != nil {
		t.Fatalf("ParseProtoFile: %v", err)
	}

	enums := filterByKind(result.Symbols, KindType)
	if len(enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(enums))
	}

	if enums[0].Name != "ServingStatus" {
		t.Errorf("enum name = %q, want %q", enums[0].Name, "ServingStatus")
	}
}

func TestDetectLanguageProto(t *testing.T) {
	if got := detectLanguage("service.proto"); got != LangProto {
		t.Errorf("detectLanguage('service.proto') = %q, want %q", got, LangProto)
	}
}

func filterByKind(symbols []Symbol, kind SymbolKind) []Symbol {
	var out []Symbol

	for _, s := range symbols {
		if s.Kind == kind {
			out = append(out, s)
		}
	}

	return out
}
