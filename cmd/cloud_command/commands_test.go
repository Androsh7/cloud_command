package main

import "testing"

func TestScriptFromArgumentsKeepsSingleArgumentAsScript(t *testing.T) {
	script := scriptFromArguments([]string{"sleep 5 && echo done | tee /tmp/out"})

	if script != "sleep 5 && echo done | tee /tmp/out" {
		t.Fatalf("got %q", script)
	}
}

func TestScriptFromArgumentsQuotesSeveralArguments(t *testing.T) {
	script := scriptFromArguments([]string{"echo", "hello world", "it's", "--flag=1", ""})

	if script != `echo 'hello world' 'it'\''s' --flag=1 ''` {
		t.Fatalf("got %q", script)
	}
}
