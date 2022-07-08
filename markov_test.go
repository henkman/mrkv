package mrkv

import (
	"fmt"
	"os"
	"testing"
)

func TestFromDBFile(t *testing.T) {
	var m Markov
	if err := m.InitFromDB("./alice.mrkv", 11111); err != nil {
		panic(err)
		return
	}
	const EXPECTED = "Whoever lives there could grin"
	chain := m.Generate(5)
	words := WordJoin(chain)
	if EXPECTED != words {
		t.Fail()
		fmt.Println("is", words, "should be", EXPECTED)
	}
}

func TestBasic(t *testing.T) {
	var m Markov
	m.Init(11111)
	{
		fd, err := os.Open("alice.txt")
		if err != nil {
			panic(err)
		}
		m.Feed(fd)
		fd.Close()
	}
	const EXPECTED = "Whoever lives there could grin"
	chain := m.Generate(5)
	words := WordJoin(chain)
	if EXPECTED != words {
		t.Fail()
		fmt.Printf("is '%s' but should be '%s'\n", words, EXPECTED)
	}
}
