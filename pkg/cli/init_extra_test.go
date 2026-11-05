package cli

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadLine_StopsAtEOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("first\nlast"))
	for _, want := range []string{"first", "last"} {
		got, err := readLine(r)
		if err != nil || got != want {
			t.Fatalf("readLine() = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := readLine(r); err == nil {
		t.Fatal("readLine() at end of input should fail instead of returning \"\" forever")
	}
}

func TestPageIDFromInput(t *testing.T) {
	id := "1e369015c2408021a930e3ee43ea0ef6"
	for _, in := range []string{
		id,
		"https://www.notion.so/myteam/Team-Wiki-" + id,
		"https://www.notion.so/" + id + "?pvs=4",
	} {
		if got := pageIDFromInput(in); got != id {
			t.Errorf("pageIDFromInput(%q) = %q, want %q", in, got, id)
		}
	}
}
