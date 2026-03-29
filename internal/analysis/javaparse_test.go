package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testJavaFile = `
package com.example.myapp;

import java.util.List;
import java.util.Map;
import static java.util.Collections.emptyList;

@SuppressWarnings("unchecked")
public class UserService {

    private final Map<String, String> cache;

    public UserService() {
        this.cache = null;
    }

    @Override
    public String getName(String id) {
        return cache.get(id);
    }

    private void reload() {
        // internal
    }

    protected int count() {
        return 0;
    }
}

public interface Repository {
    List<String> findAll();
}

enum Status {
    ACTIVE,
    INACTIVE
}

abstract class BaseHandler {
    public abstract void handle();
}

class DefaultHandler extends BaseHandler {
    public void handle() {}
}
`

const testKotlinFile = `
package com.example.app

import kotlinx.coroutines.flow.Flow
import com.example.model.User

@Serializable
data class Config(val host: String, val port: Int)

sealed class Result {
    data class Success(val value: String) : Result()
    data class Error(val message: String) : Result()
}

interface UserRepository {
    fun findById(id: String): User?
    fun findAll(): List<User>
}

enum class Direction {
    NORTH, SOUTH, EAST, WEST
}

object AppLogger {
    fun log(msg: String) {}
}

open class Service {
    fun start() {}
    private fun stop() {}
    suspend fun process(data: String): Result {
        return Result.Success(data)
    }
}

internal class InternalHelper {
    fun help() {}
}
`

func writeTempJavaFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "UserService.java")

	if err := os.WriteFile(path, []byte(testJavaFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func writeTempKotlinFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "App.kt")

	if err := os.WriteFile(path, []byte(testKotlinFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParseJavaFilePackage(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	if result.Package != "com.example.myapp" {
		t.Errorf("Package = %q, want %q", result.Package, "com.example.myapp")
	}

	pkgs := filterByKind(result.Symbols, KindPackage)
	if len(pkgs) != 1 || pkgs[0].Name != "com.example.myapp" {
		t.Errorf("expected package symbol com.example.myapp, got %v", pkgs)
	}
}

func TestParseJavaFileImports(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	if len(result.Imports) != 3 {
		t.Fatalf("expected 3 imports, got %d: %v", len(result.Imports), result.Imports)
	}

	expected := []string{"java.util.List", "java.util.Map", "java.util.Collections.emptyList"}
	for i, want := range expected {
		if result.Imports[i] != want {
			t.Errorf("import[%d] = %q, want %q", i, result.Imports[i], want)
		}
	}
}

func TestParseJavaFileClasses(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	types := filterByKind(result.Symbols, KindType)

	names := make(map[string]bool)
	for _, ty := range types {
		names[ty.Name] = true
	}

	for _, want := range []string{"UserService", "BaseHandler", "DefaultHandler", "Status"} {
		if !names[want] {
			t.Errorf("missing type: %s", want)
		}
	}

	// Check that abstract class has correct signature.
	for _, ty := range types {
		if ty.Name == "BaseHandler" {
			if ty.Signature != "abstract class BaseHandler" {
				t.Errorf("BaseHandler signature = %q, want %q", ty.Signature, "abstract class BaseHandler")
			}
		}
	}
}

func TestParseJavaFileInterfaces(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	ifaces := filterByKind(result.Symbols, KindInterface)

	if len(ifaces) != 1 || ifaces[0].Name != "Repository" {
		t.Errorf("expected interface Repository, got %v", ifaces)
	}
}

func TestParseJavaFileMethods(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	methods := filterByKind(result.Symbols, KindMethod)

	names := make(map[string]bool)
	exported := make(map[string]bool)

	for _, m := range methods {
		names[m.Name] = true
		exported[m.Name] = m.Exported
	}

	for _, want := range []string{"getName", "reload", "count"} {
		if !names[want] {
			t.Errorf("missing method: %s", want)
		}
	}

	// Check visibility.
	if exported["reload"] {
		t.Error("reload should not be exported (private)")
	}

	if !exported["getName"] {
		t.Error("getName should be exported (public)")
	}
}

func TestParseJavaFileEnums(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	types := filterByKind(result.Symbols, KindType)

	found := false

	for _, ty := range types {
		if ty.Name == "Status" {
			found = true

			if ty.Signature != "enum Status" {
				t.Errorf("Status signature = %q, want %q", ty.Signature, "enum Status")
			}
		}
	}

	if !found {
		t.Error("missing enum: Status")
	}
}

func TestParseJavaFileLanguage(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	if result.Language != LangJava {
		t.Errorf("Language = %q, want %q", result.Language, LangJava)
	}
}

func TestParseJavaFileAnnotations(t *testing.T) {
	path := writeTempJavaFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	// The @SuppressWarnings annotation should be attached to UserService.
	for _, sym := range result.Symbols {
		if sym.Name == "UserService" && sym.Kind == KindType {
			if sym.Doc != "@SuppressWarnings" {
				t.Errorf("UserService doc = %q, want %q", sym.Doc, "@SuppressWarnings")
			}

			return
		}
	}

	t.Error("UserService type symbol not found")
}

// Kotlin tests.

func TestParseKotlinFilePackage(t *testing.T) {
	path := writeTempKotlinFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile (kotlin): %v", err)
	}

	if result.Package != "com.example.app" {
		t.Errorf("Package = %q, want %q", result.Package, "com.example.app")
	}
}

func TestParseKotlinFileImports(t *testing.T) {
	path := writeTempKotlinFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile (kotlin): %v", err)
	}

	if len(result.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d: %v", len(result.Imports), result.Imports)
	}
}

func TestParseKotlinFileClasses(t *testing.T) {
	path := writeTempKotlinFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile (kotlin): %v", err)
	}

	types := filterByKind(result.Symbols, KindType)

	names := make(map[string]bool)
	sigs := make(map[string]string)

	for _, ty := range types {
		names[ty.Name] = true
		sigs[ty.Name] = ty.Signature
	}

	for _, want := range []string{"Config", "Result", "Direction", "AppLogger", "Service", "InternalHelper"} {
		if !names[want] {
			t.Errorf("missing type: %s", want)
		}
	}

	if sigs["Config"] != "data class Config" {
		t.Errorf("Config signature = %q, want %q", sigs["Config"], "data class Config")
	}

	if sigs["Result"] != "sealed class Result" {
		t.Errorf("Result signature = %q, want %q", sigs["Result"], "sealed class Result")
	}
}

func TestParseKotlinFileInterfaces(t *testing.T) {
	path := writeTempKotlinFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile (kotlin): %v", err)
	}

	ifaces := filterByKind(result.Symbols, KindInterface)

	if len(ifaces) != 1 || ifaces[0].Name != "UserRepository" {
		t.Errorf("expected interface UserRepository, got %v", ifaces)
	}
}

func TestParseKotlinFileEnums(t *testing.T) {
	path := writeTempKotlinFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile (kotlin): %v", err)
	}

	types := filterByKind(result.Symbols, KindType)

	found := false

	for _, ty := range types {
		if ty.Name == "Direction" {
			found = true

			if ty.Signature != "enum class Direction" {
				t.Errorf("Direction signature = %q, want %q", ty.Signature, "enum class Direction")
			}
		}
	}

	if !found {
		t.Error("missing enum: Direction")
	}
}

func TestParseKotlinFileFunctions(t *testing.T) {
	path := writeTempKotlinFile(t)

	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile (kotlin): %v", err)
	}

	fns := filterByKind(result.Symbols, KindFunction)

	names := make(map[string]bool)
	for _, f := range fns {
		names[f.Name] = true
	}

	// Top-level interface methods are functions (braceDepth==0 inside interface braces handled by brace tracking).
	// Service.start, Service.stop, Service.process are methods (inside class).
	methods := filterByKind(result.Symbols, KindMethod)

	methodNames := make(map[string]bool)
	for _, m := range methods {
		methodNames[m.Name] = true
	}

	for _, want := range []string{"start", "stop", "process"} {
		if !methodNames[want] {
			t.Errorf("missing method: %s", want)
		}
	}

	// Check private visibility.
	for _, m := range methods {
		if m.Name == "stop" && m.Exported {
			t.Error("stop should not be exported (private)")
		}
	}
}

func TestDetectLanguageJava(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"Main.java", LangJava},
		{"App.kt", LangJava},
		{"build.gradle.kts", LangJava},
		{"other.txt", ""},
	}

	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
