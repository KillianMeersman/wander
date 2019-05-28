package util_test

import (
	"math/rand"
	"testing"

	"github.com/KillianMeersman/wander/util"
)

func randomStrings(n int) []string {
	randStrings := make([]string, n)
	for i := 0; i < n; i++ {
		randStrings[i] = util.RandomString(rand.Intn(100))
	}
	return randStrings
}

func TestStringDoubleLinkedList(t *testing.T) {
	randStrings := randomStrings(1000)
	list := util.NewStringLinkedList()

	for _, str := range randStrings {
		list.Add(str)
	}
	if list.Count() != 1000 {
		t.Fatal("list count is not 1000")
	}

	for i := 999; i >= 0; i-- {
		next := list.Pop()
		if randStrings[i] != next {
			t.Fatalf("Pop should have yielded %s, instead %s", randStrings[i], next)
		}
	}

	if list.Count() != 0 {
		t.Fatal("list count is not 0")
	}
}
