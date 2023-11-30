package utils_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils"
)

// test Render method happy path
func TestBlackfridayRenderer_Render(t *testing.T) {
	//create a renderer
	renderer := utils.BlackfridayRenderer{}
	//call Render method
	source := []byte("test")
	output, _ := renderer.Render(source)
	// add your assertions here
	//verify that the method returned the expected value

	expected := []byte("<p>test</p>\n")
	if string(expected) != string(output) {
		t.Errorf("expected: %s, got: %s", expected, output)
	}
}
