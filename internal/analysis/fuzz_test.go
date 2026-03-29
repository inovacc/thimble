package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

// fuzzWriteTempFile writes data to a temp file with the given extension and returns the path.
func fuzzWriteTempFile(t *testing.T, ext string, data []byte) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "fuzz-*"+ext)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()

		t.Fatal(err)
	}

	_ = f.Close()

	return f.Name()
}

// --- Seed corpora ---------------------------------------------------------

var seedGo = []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

var seedPython = []byte(`import os

class Greeter:
    def greet(self, name: str) -> str:
        return f"hello {name}"

def main():
    g = Greeter()
    print(g.greet("world"))
`)

var seedTS = []byte(`export function greet(name: string): string {
  return "hello " + name;
}

export class Greeter {
  greet(name: string): string {
    return "hello " + name;
  }
}
`)

var seedRust = []byte(`use std::fmt;

pub struct Greeter;

impl Greeter {
    pub fn greet(&self, name: &str) -> String {
        format!("hello {}", name)
    }
}

fn main() {
    let g = Greeter;
    println!("{}", g.greet("world"));
}
`)

var seedProto = []byte(`syntax = "proto3";

package example;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
`)

var seedShell = []byte(`#!/bin/bash
set -euo pipefail

readonly VERSION="1.0.0"

function greet() {
    echo "hello $1"
}

main() {
    greet "world"
}

main "$@"
`)

var seedC = []byte(`#include <stdio.h>

#define MAX_LEN 256

struct Greeter {
    char name[MAX_LEN];
};

void greet(const char *name) {
    printf("hello %s\n", name);
}

int main(void) {
    greet("world");
    return 0;
}
`)

var seedJava = []byte(`package com.example;

import java.util.List;

public class Greeter {
    public String greet(String name) {
        return "hello " + name;
    }

    public static void main(String[] args) {
        Greeter g = new Greeter();
        System.out.println(g.greet("world"));
    }
}
`)

// --- Fuzz functions -------------------------------------------------------

func FuzzParseGoFile(f *testing.F) {
	f.Add(seedGo)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".go", data)
		_, _ = ParseGoFile(path)
	})
}

func FuzzParsePythonFile(f *testing.F) {
	f.Add(seedPython)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".py", data)
		_, _ = ParsePythonFile(path)
	})
}

func FuzzParseTSFile(f *testing.F) {
	f.Add(seedTS)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".ts", data)
		_, _ = ParseTSFile(path)
	})
}

func FuzzParseRustFile(f *testing.F) {
	f.Add(seedRust)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".rs", data)
		_, _ = ParseRustFile(path)
	})
}

func FuzzParseProtoFile(f *testing.F) {
	f.Add(seedProto)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".proto", data)
		_, _ = ParseProtoFile(path)
	})
}

func FuzzParseShellFile(f *testing.F) {
	f.Add(seedShell)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".sh", data)
		_, _ = ParseShellFile(path)
	})
}

func FuzzParseCFile(f *testing.F) {
	f.Add(seedC)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := fuzzWriteTempFile(t, ".c", data)
		_, _ = ParseCFile(path)
	})
}

func FuzzParseJavaFile(f *testing.F) {
	f.Add(seedJava)
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "Fuzz.java")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, _ = ParseJavaFile(path)
	})
}
