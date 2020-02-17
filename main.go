package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	stdinFlag := flag.String("stdin", "", "Override standard input.")
	stdinAddFlag := flag.String("stdin_add", "", "Append to standard input.")
	flag.Parse()

	var stdin io.Reader
	stdin = os.Stdin
	if len(*stdinFlag) != 0 {
		stdin = strings.NewReader(*stdinFlag)
	}
	if len(*stdinAddFlag) != 0 {
		stdin = io.MultiReader(stdin, strings.NewReader(*stdinAddFlag))
	}

	// Parse all files given by the command line arguments.
	stack := make([]string, 0)
	tokens := make([]Token, 0)
	global := MakeSet() // Global names
	loaded := MakeSet() // Already parsed files

	for _, arg := range flag.Args() {
		path, _ := filepath.Abs(arg)
		stack = append(stack, path)
	}

	for len(stack) > 0 {
		var path string
		path, stack = stack[len(stack)-1], stack[:len(stack)-1]
		if loaded.Contains(path) {
			continue
		}
		loaded.Add(path)

		// Try to read file.
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		// Extract directives.
		attachAdd, globalAdd, offset, source := ExtractDirectives(string(bytes))
		global.AddAll(castStrSliceToInterface(globalAdd)...)

		// Add attached files relative to this file.
		for _, attachment := range attachAdd {
			abs, _ := filepath.Abs(filepath.Join(filepath.Dir(path), attachment))
			stack = append(stack, abs)
		}

		// Add tokens in this file.
		tokens = append(tokens, Tokenize(source, Loc{path, offset + 1, 0}, true)...)
	}

	// Wrap all processes in globally defined names.
	full := make([]Token, 0, len(tokens)+2*len(global)+2)
	for v := range global {
		full = append(full, Token{Loc{}, fmt.Sprintf("+%v", v)}, Token{Loc{}, ";"})
	}
	full = append(full, Token{Loc{}, "("})
	full = append(full, tokens...)
	full = append(full, Token{Loc{}, ")"})

	// Parse program.
	err := ErrorList([]error{})
	proc, unparsed := Parse(full, ioChannelOffset, copyStrIntMap(nil), &err)
	if len(unparsed) > 0 {
		fmt.Printf("%v tokens were not parsed", len(unparsed))
		return
	} else if len(err) != 0 {
		for _, e := range err {
			println(e.Error())
		}
		return
	}

	// Optimize program.
	proc = Optimize(proc)

	// Run program.
	pi := Pi{0, nil, nil, nil}
	pi.Initialize(proc)
	for len(pi.Queue)+len(pi.Ether) > 0 {
		for len(pi.Queue) > 0 {
			pi.RunNextNode()
		}
		pi.DeliverMessages(stdin, os.Stdout)
	}
}
