package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testTSFile = `import { Request, Response } from 'express';
import path from 'node:path';
import './styles.css';

export interface UserService {
  getUser(id: string): Promise<User>;
  listUsers(): Promise<User[]>;
}

export interface Serializable extends Base {
  serialize(): string;
}

export type UserID = string;
export type Config = { port: number; host: string };

export class UserController extends BaseController implements UserService {
  private db: Database;
}

export abstract class Repository {
  abstract find(id: string): Promise<any>;
}

export function createApp(config: Config): Application {
  return new Application(config);
}

export async function fetchUsers(limit: number): Promise<User[]> {
  return [];
}

export const API_VERSION = '2.0';
export const MAX_RETRIES: number = 3;

export enum Role {
  Admin = 'admin',
  User = 'user',
}

export default class App {
  start(): void {}
}
`

func writeTempTSFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "app.ts")
	if err := os.WriteFile(path, []byte(testTSFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParseTSFileImports(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	imports := make(map[string]bool)
	for _, imp := range result.Imports {
		imports[imp] = true
	}

	if !imports["express"] {
		t.Error("missing import: express")
	}

	if !imports["node:path"] {
		t.Error("missing import: node:path")
	}

	if !imports["./styles.css"] {
		t.Error("missing import: ./styles.css")
	}
}

func TestParseTSFileInterfaces(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	ifaces := filterByKind(result.Symbols, KindInterface)
	if len(ifaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(ifaces))
	}

	names := make(map[string]bool)
	for _, s := range ifaces {
		names[s.Name] = true
	}

	if !names["UserService"] {
		t.Error("missing interface: UserService")
	}

	if !names["Serializable"] {
		t.Error("missing interface: Serializable")
	}
}

func TestParseTSFileClasses(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	classes := filterByKind(result.Symbols, KindStruct)
	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}

	// Check UserController has extends + implements.
	for _, c := range classes {
		if c.Name == "UserController" {
			if c.Signature != "class UserController extends BaseController implements UserService" {
				t.Errorf("UserController signature = %q", c.Signature)
			}
		}
	}
}

func TestParseTSFileFunctions(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	funcs := filterByKind(result.Symbols, KindFunction)
	// createApp, fetchUsers, App (default export)
	if len(funcs) < 2 {
		t.Fatalf("expected at least 2 functions, got %d", len(funcs))
	}

	names := make(map[string]bool)
	for _, f := range funcs {
		names[f.Name] = true
	}

	if !names["createApp"] {
		t.Error("missing function: createApp")
	}

	if !names["fetchUsers"] {
		t.Error("missing function: fetchUsers")
	}
}

func TestParseTSFileConstants(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	consts := filterByKind(result.Symbols, KindConstant)
	if len(consts) != 2 {
		t.Fatalf("expected 2 constants, got %d", len(consts))
	}

	names := make(map[string]bool)
	for _, c := range consts {
		names[c.Name] = true
	}

	if !names["API_VERSION"] {
		t.Error("missing constant: API_VERSION")
	}

	if !names["MAX_RETRIES"] {
		t.Error("missing constant: MAX_RETRIES")
	}
}

func TestParseTSFileTypes(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	types := filterByKind(result.Symbols, KindType)
	// UserID, Config, Role (enum)
	if len(types) != 3 {
		t.Fatalf("expected 3 types (2 aliases + 1 enum), got %d", len(types))
	}
}

func TestParseTSFileAllExported(t *testing.T) {
	path := writeTempTSFile(t)

	result, err := ParseTSFile(path)
	if err != nil {
		t.Fatalf("ParseTSFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if !sym.Exported {
			t.Errorf("symbol %q should be exported", sym.Name)
		}
	}
}

func TestDetectLanguageTS(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"app.ts", LangTypeScript},
		{"app.tsx", LangTypeScript},
		{"app.js", LangTypeScript},
		{"app.jsx", LangTypeScript},
		{"app.mjs", LangTypeScript},
	}
	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
