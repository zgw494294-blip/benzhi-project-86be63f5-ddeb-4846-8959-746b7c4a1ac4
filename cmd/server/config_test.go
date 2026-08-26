package main

import "testing"

func TestResolveAddressPrecedence(t *testing.T) {
	t.Setenv("PORT", "20123")
	got, err := resolveAddress("")
	if err != nil || got != "127.0.0.1:20123" {
		t.Fatalf("PORT 解析为 %q, %v", got, err)
	}
	got, err = resolveAddress("127.0.0.1:20456")
	if err != nil || got != "127.0.0.1:20456" {
		t.Fatalf("-addr 未优先: %q, %v", got, err)
	}
}

func TestResolveAddressRejectsPublicAndInvalidPort(t *testing.T) {
	if _, err := resolveAddress("0.0.0.0:19081"); err == nil {
		t.Fatal("应拒绝公开监听")
	}
	t.Setenv("PORT", "70000")
	if _, err := resolveAddress(""); err == nil {
		t.Fatal("应拒绝非法 PORT")
	}
}
