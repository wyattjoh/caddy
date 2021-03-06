package caddyfile

import (
	"os"
	"strings"
	"testing"
)

func TestAllTokens(t *testing.T) {
	input := strings.NewReader("a b c\nd e")
	expected := []string{"a", "b", "c", "d", "e"}
	tokens := allTokens(input)

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, val := range expected {
		if tokens[i].Text != val {
			t.Errorf("Token %d should be '%s' but was '%s'", i, val, tokens[i].Text)
		}
	}
}

func TestParseOneAndImport(t *testing.T) {
	testParseOne := func(input string) (ServerBlock, error) {
		p := testParser(input)
		p.Next() // parseOne doesn't call Next() to start, so we must
		err := p.parseOne()
		return p.block, err
	}

	for i, test := range []struct {
		input     string
		shouldErr bool
		keys      []string
		tokens    map[string]int // map of directive name to number of tokens expected
	}{
		{`localhost`, false, []string{
			"localhost",
		}, map[string]int{}},

		{`localhost
		  dir1`, false, []string{
			"localhost",
		}, map[string]int{
			"dir1": 1,
		}},

		{`localhost:1234
		  dir1 foo bar`, false, []string{
			"localhost:1234",
		}, map[string]int{
			"dir1": 3,
		}},

		{`localhost {
		    dir1
		  }`, false, []string{
			"localhost",
		}, map[string]int{
			"dir1": 1,
		}},

		{`localhost:1234 {
		    dir1 foo bar
		    dir2
		  }`, false, []string{
			"localhost:1234",
		}, map[string]int{
			"dir1": 3,
			"dir2": 1,
		}},

		{`http://localhost https://localhost
		  dir1 foo bar`, false, []string{
			"http://localhost",
			"https://localhost",
		}, map[string]int{
			"dir1": 3,
		}},

		{`http://localhost https://localhost {
		    dir1 foo bar
		  }`, false, []string{
			"http://localhost",
			"https://localhost",
		}, map[string]int{
			"dir1": 3,
		}},

		{`http://localhost, https://localhost {
		    dir1 foo bar
		  }`, false, []string{
			"http://localhost",
			"https://localhost",
		}, map[string]int{
			"dir1": 3,
		}},

		{`http://localhost, {
		  }`, true, []string{
			"http://localhost",
		}, map[string]int{}},

		{`host1:80, http://host2.com
		  dir1 foo bar
		  dir2 baz`, false, []string{
			"host1:80",
			"http://host2.com",
		}, map[string]int{
			"dir1": 3,
			"dir2": 2,
		}},

		{`http://host1.com,
		  http://host2.com,
		  https://host3.com`, false, []string{
			"http://host1.com",
			"http://host2.com",
			"https://host3.com",
		}, map[string]int{}},

		{`http://host1.com:1234, https://host2.com
		  dir1 foo {
		    bar baz
		  }
		  dir2`, false, []string{
			"http://host1.com:1234",
			"https://host2.com",
		}, map[string]int{
			"dir1": 6,
			"dir2": 1,
		}},

		{`127.0.0.1
		  dir1 {
		    bar baz
		  }
		  dir2 {
		    foo bar
		  }`, false, []string{
			"127.0.0.1",
		}, map[string]int{
			"dir1": 5,
			"dir2": 5,
		}},

		{`localhost
		  dir1 {
		    foo`, true, []string{
			"localhost",
		}, map[string]int{
			"dir1": 3,
		}},

		{`localhost
		  dir1 {
		  }`, false, []string{
			"localhost",
		}, map[string]int{
			"dir1": 3,
		}},

		{`localhost
		  dir1 {
		  } }`, true, []string{
			"localhost",
		}, map[string]int{
			"dir1": 3,
		}},

		{`localhost
		  dir1 {
		    nested {
		      foo
		    }
		  }
		  dir2 foo bar`, false, []string{
			"localhost",
		}, map[string]int{
			"dir1": 7,
			"dir2": 3,
		}},

		{``, false, []string{}, map[string]int{}},

		{`localhost
		  dir1 arg1
		  import ../testdata/import_test1.txt`, false, []string{
			"localhost",
		}, map[string]int{
			"dir1": 2,
			"dir2": 3,
			"dir3": 1,
		}},

		{`import ../testdata/import_test2.txt`, false, []string{
			"host1",
		}, map[string]int{
			"dir1": 1,
			"dir2": 2,
		}},

		{`import ../testdata/import_test1.txt ../testdata/import_test2.txt`, true, []string{}, map[string]int{}},

		{`import ../testdata/not_found.txt`, true, []string{}, map[string]int{}},

		{`""`, false, []string{}, map[string]int{}},

		{``, false, []string{}, map[string]int{}},
	} {
		result, err := testParseOne(test.input)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected an error, but didn't get one", i)
		}
		if !test.shouldErr && err != nil {
			t.Errorf("Test %d: Expected no error, but got: %v", i, err)
		}

		if len(result.Keys) != len(test.keys) {
			t.Errorf("Test %d: Expected %d keys, got %d",
				i, len(test.keys), len(result.Keys))
			continue
		}
		for j, addr := range result.Keys {
			if addr != test.keys[j] {
				t.Errorf("Test %d, key %d: Expected '%s', but was '%s'",
					i, j, test.keys[j], addr)
			}
		}

		if len(result.Tokens) != len(test.tokens) {
			t.Errorf("Test %d: Expected %d directives, had %d",
				i, len(test.tokens), len(result.Tokens))
			continue
		}
		for directive, tokens := range result.Tokens {
			if len(tokens) != test.tokens[directive] {
				t.Errorf("Test %d, directive '%s': Expected %d tokens, counted %d",
					i, directive, test.tokens[directive], len(tokens))
				continue
			}
		}
	}
}

func TestParseAll(t *testing.T) {
	for i, test := range []struct {
		input     string
		shouldErr bool
		keys      [][]string // keys per server block, in order
	}{
		{`localhost`, false, [][]string{
			{"localhost"},
		}},

		{`localhost:1234`, false, [][]string{
			{"localhost:1234"},
		}},

		{`localhost:1234 {
		  }
		  localhost:2015 {
		  }`, false, [][]string{
			{"localhost:1234"},
			{"localhost:2015"},
		}},

		{`localhost:1234, http://host2`, false, [][]string{
			{"localhost:1234", "http://host2"},
		}},

		{`localhost:1234, http://host2,`, true, [][]string{}},

		{`http://host1.com, http://host2.com {
		  }
		  https://host3.com, https://host4.com {
		  }`, false, [][]string{
			{"http://host1.com", "http://host2.com"},
			{"https://host3.com", "https://host4.com"},
		}},

		{`import testdata/import_glob*.txt`, false, [][]string{
			{"glob0.host0"},
			{"glob0.host1"},
			{"glob1.host0"},
			{"glob2.host0"},
		}},
	} {
		p := testParser(test.input)
		blocks, err := p.parseAll()

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected an error, but didn't get one", i)
		}
		if !test.shouldErr && err != nil {
			t.Errorf("Test %d: Expected no error, but got: %v", i, err)
		}

		if len(blocks) != len(test.keys) {
			t.Errorf("Test %d: Expected %d server blocks, got %d",
				i, len(test.keys), len(blocks))
			continue
		}
		for j, block := range blocks {
			if len(block.Keys) != len(test.keys[j]) {
				t.Errorf("Test %d: Expected %d keys in block %d, got %d",
					i, len(test.keys[j]), j, len(block.Keys))
				continue
			}
			for k, addr := range block.Keys {
				if addr != test.keys[j][k] {
					t.Errorf("Test %d, block %d, key %d: Expected '%s', but got '%s'",
						i, j, k, test.keys[j][k], addr)
				}
			}
		}
	}
}

func TestEnvironmentReplacement(t *testing.T) {
	os.Setenv("PORT", "8080")
	os.Setenv("ADDRESS", "servername.com")
	os.Setenv("FOOBAR", "foobar")

	// basic test; unix-style env vars
	p := testParser(`{$ADDRESS}`)
	blocks, _ := p.parseAll()
	if actual, expected := blocks[0].Keys[0], "servername.com"; expected != actual {
		t.Errorf("Expected key to be '%s' but was '%s'", expected, actual)
	}

	// multiple vars per token
	p = testParser(`{$ADDRESS}:{$PORT}`)
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Keys[0], "servername.com:8080"; expected != actual {
		t.Errorf("Expected key to be '%s' but was '%s'", expected, actual)
	}

	// windows-style var and unix style in same token
	p = testParser(`{%ADDRESS%}:{$PORT}`)
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Keys[0], "servername.com:8080"; expected != actual {
		t.Errorf("Expected key to be '%s' but was '%s'", expected, actual)
	}

	// reverse order
	p = testParser(`{$ADDRESS}:{%PORT%}`)
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Keys[0], "servername.com:8080"; expected != actual {
		t.Errorf("Expected key to be '%s' but was '%s'", expected, actual)
	}

	// env var in server block body as argument
	p = testParser(":{%PORT%}\ndir1 {$FOOBAR}")
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Keys[0], ":8080"; expected != actual {
		t.Errorf("Expected key to be '%s' but was '%s'", expected, actual)
	}
	if actual, expected := blocks[0].Tokens["dir1"][1].Text, "foobar"; expected != actual {
		t.Errorf("Expected argument to be '%s' but was '%s'", expected, actual)
	}

	// combined windows env vars in argument
	p = testParser(":{%PORT%}\ndir1 {%ADDRESS%}/{%FOOBAR%}")
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Tokens["dir1"][1].Text, "servername.com/foobar"; expected != actual {
		t.Errorf("Expected argument to be '%s' but was '%s'", expected, actual)
	}

	// malformed env var (windows)
	p = testParser(":1234\ndir1 {%ADDRESS}")
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Tokens["dir1"][1].Text, "{%ADDRESS}"; expected != actual {
		t.Errorf("Expected host to be '%s' but was '%s'", expected, actual)
	}

	// malformed (non-existent) env var (unix)
	p = testParser(`:{$PORT$}`)
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Keys[0], ":"; expected != actual {
		t.Errorf("Expected key to be '%s' but was '%s'", expected, actual)
	}

	// in quoted field
	p = testParser(":1234\ndir1 \"Test {$FOOBAR} test\"")
	blocks, _ = p.parseAll()
	if actual, expected := blocks[0].Tokens["dir1"][1].Text, "Test foobar test"; expected != actual {
		t.Errorf("Expected argument to be '%s' but was '%s'", expected, actual)
	}
}

func testParser(input string) parser {
	buf := strings.NewReader(input)
	// use relative path to test that import paths are relative to
	// the Caddyfile rather than the current working directory (issue #867)
	p := parser{Dispenser: NewDispenser("../Test", buf)}
	return p
}
