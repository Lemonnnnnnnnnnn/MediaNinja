package parsers

import (
	"MediaNinja/core/request/client"
	"testing"
)

func TestRule34OptionsAreTaskLocal(t *testing.T) {
	c := client.NewClient("", 1, 0)
	first := GetParserWithOptions("https://rule34video.com/models/one/", c, Rule34ListOptions{MaxPages: 2, Concurrency: 3}).(*Rule34VideoParser)
	second := GetParserWithOptions("https://rule34video.com/models/two/", c, Rule34ListOptions{MaxPages: 9, Concurrency: 7}).(*Rule34VideoParser)
	second.options.MaxPages = 20
	if first.options.MaxPages != 2 || first.options.Concurrency != 3 {
		t.Fatal("options leaked across tasks")
	}
}
