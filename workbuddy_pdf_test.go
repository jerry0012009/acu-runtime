package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkBuddyPDFRepairPreservesPermissionsAndOverrides(t *testing.T) {
	t.Setenv("WORKBUDDY_PDF", "")
	for _, initial := range []string{
		`{"sandbox":{"enabled":true},"securityReview":{"enabled":false},"unknown":[1,2],"env":{"OTHER":"value"}}`,
		`{"env":{"WORKBUDDY_PDF":"custom.exe"},"sandbox":{"enabled":true}}`,
		`{"env":{"WORKBUDDY_PDF":""}}`,
	} {
		dir := t.TempDir()
		cli := filepath.Join(dir, "external-packages", "pdf-cli", "workbuddy-pdf.exe")
		os.MkdirAll(filepath.Dir(cli), 0700)
		os.WriteFile(cli, []byte("fixture, never executed"), 0600)
		path := filepath.Join(dir, "settings.json")
		os.WriteFile(path, []byte(initial), 0600)
		if err := prepareWorkBuddyPDFIn(dir); err != nil {
			t.Fatal(err)
		}
		first, _ := os.ReadFile(path)
		if err := prepareWorkBuddyPDFIn(dir); err != nil {
			t.Fatal(err)
		}
		second, _ := os.ReadFile(path)
		if string(first) != string(second) {
			t.Fatal("repair was not idempotent")
		}
		var before, after map[string]json.RawMessage
		json.Unmarshal([]byte(initial), &before)
		json.Unmarshal(first, &after)
		for k, v := range before {
			if k == "env" {
				continue
			}
			var a, b any
			json.Unmarshal(v, &a)
			json.Unmarshal(after[k], &b)
			av, _ := json.Marshal(a)
			bv, _ := json.Marshal(b)
			if string(av) != string(bv) {
				t.Fatalf("changed unrelated field %s", k)
			}
		}
		var env, oldEnv map[string]string
		json.Unmarshal(after["env"], &env)
		json.Unmarshal(before["env"], &oldEnv)
		for k, v := range oldEnv {
			if env[k] != v {
				t.Fatal("changed user environment")
			}
		}
		if _, explicit := oldEnv["WORKBUDDY_PDF"]; !explicit && env["WORKBUDDY_PDF"] != cli {
			t.Fatal("did not point official launcher at existing component")
		}
	}
}

func TestWorkBuddyPDFMissingComponentAndInvalidConfig(t *testing.T) {
	t.Setenv("WORKBUDDY_PDF", "")
	dir := t.TempDir()
	if err := prepareWorkBuddyPDFIn(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Fatal("created configuration without component")
	}
	cli := filepath.Join(dir, "external-packages", "pdf-cli", "workbuddy-pdf.exe")
	os.MkdirAll(filepath.Dir(cli), 0700)
	os.WriteFile(cli, []byte("fixture"), 0600)
	for _, invalid := range []string{`{`, `null`, `{"env":[]}`, `{"env":null}`} {
		path := filepath.Join(dir, "settings.json")
		os.WriteFile(path, []byte(invalid), 0600)
		if err := prepareWorkBuddyPDFIn(dir); err == nil {
			t.Fatal("accepted invalid settings")
		}
		data, _ := os.ReadFile(path)
		if string(data) != invalid {
			t.Fatal("overwrote invalid settings")
		}
	}
}
