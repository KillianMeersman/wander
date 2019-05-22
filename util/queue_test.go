package util_test

import (
	"log"
	"testing"

	"github.com/KillianMeersman/wander/util"
)

func testStringQueue(t *testing.T, queue util.StringQueue) {
	randStrings := randomStrings(1000)

	for _, str := range randStrings {
		err := queue.Enqueue(str)
		if err != nil {
			t.Fatal(err)
		}
	}
	if queue.Count() != 1000 {
		log.Fatal("size not 1000")
	}

	for _, str := range randStrings {
		next, ok := queue.Dequeue()
		if !ok {
			t.Fatal("could not dequeue")
		}
		if str != next {
			t.Fatalf("Dequeue should have yielded %s, instead %s", str, next)
		}
	}
	if queue.Count() != 0 {
		log.Fatal("size not 0")
	}
}

func TestStringCircularBuffer(t *testing.T) {
	testStringQueue(t, util.NewCircularStringBuffer(100, 1000))
}
