package limits_test

import (
	"strings"
	"testing"

	"github.com/KillianMeersman/wander/limits"
)

var robots string = `
User-agent: *
Disallow:

# too many repeated hits, too quick
User-agent: litefinder
Disallow: /

# Yahoo. too many repeated hits, too quick
User-agent: Slurp
Disallow: /
Allow: /test

# too many repeated hits, too quick
User-agent: Baidu
Disallow: /
`

func TestRobotLimits(t *testing.T) {
	reader := strings.NewReader(robots)

	limits, err := limits.ParseRobotLimits(reader)
	if err != nil {
		t.Fatal(err)
	}

	if limits.Allowed("Baidu", "/") {
		t.Fatal("Baidu should not be allowed")
	}
	if limits.Allowed("Slurp", "/tess") {
		t.Fatal("Slurp should not be allowed")
	}
	if !limits.Allowed("Slurp", "/test/1") {
		t.Fatal("Slurp be allowed to access /test/1")
	}
	if !limits.Allowed("PriceTracker/0.1", "/robots.txt") {
		t.Fatal("PriceTracker/0.1 should be allowed")
	}
}

func TestMatchURL(t *testing.T) {
	if !limits.MatchURLRule("/*/*/test", "/hello/world/test") {
		t.FailNow()
	}
	if limits.MatchURLRule("/*/*/test", "/hello/test/ssfs") {
		t.FailNow()
	}
	if limits.MatchURLRule("/*?", "/test/is/nice") {
		t.FailNow()
	}
	if !limits.MatchURLRule("/*?", "/test/is/nice?param=1") {
		t.FailNow()
	}
	if limits.MatchURLRule("/*?$", "/test/is/nice?param=1") {
		t.FailNow()
	}
	if !limits.MatchURLRule("/*?$", "/test/is/nice?") {
		t.FailNow()
	}
	if !limits.MatchURLRule("/*/*/test$", "/test1/test$/test") {
		t.FailNow()
	}
	if !limits.MatchURLRule("/*/*/*", "/test1/test$/test") {
		t.FailNow()
	}
	if limits.MatchURLRule("/test1/test2/*?", "/") {
		t.FailNow()
	}
	if limits.MatchURLRule("/", "") {
		t.FailNow()
	}
	if !limits.MatchURLRule("", "/") {
		t.FailNow()
	}
	if !limits.MatchURLRule("", "") {
		t.FailNow()
	}
	if limits.MatchURLRule("/*/?z=1", "/test/") {
		t.FailNow()
	}
	if !limits.MatchURLRule("/*/?z=1", "/test/?z=1") {
		t.FailNow()
	}
}
