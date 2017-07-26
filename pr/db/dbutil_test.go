package db

import "testing"

func TestParseMaintainerString(t *testing.T) {
	l2dy := parseMaintainerString("l2dy @l2dy")
	if l2dy.Email != "l2dy@macports.org" {
		t.Error("Expected @macports.org email, got", l2dy.Email)
	}
	if l2dy.GithubHandle != "l2dy" {
		t.Error("Expected GitHub login, got", l2dy.GithubHandle)
	}
	jverne := parseMaintainerString("@jverne example.org:julesverne")
	if jverne.Email != "julesverne@example.org" {
		t.Error("Expected deobfuscated email, got", jverne.Email)
	}
}
