package util_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/KillianMeersman/wander/util"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func randomStrings(n int) []string {
	randStrings := make([]string, n)
	for i := 0; i < n; i++ {
		randStrings[i] = randomString(rand.Intn(100))
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
