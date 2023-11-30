package utils_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils"
)

// test Render method happy path
func TestGoldmarkRenderer_Render(t *testing.T) {
	//create a renderer
	renderer := utils.GoldmarkRenderer{}
	//call Render method
	source := []byte("test")
	output, err := renderer.Render(source)
	// add your assertions here
	//verify that the method returned the expected value

	if err != nil {
		t.Error(err)
	}

	expected := []byte("<p>test</p>\n")
	if string(expected) != string(output) {
		t.Errorf("expected: %s, got: %s", expected, output)
	}
}

func TestGoldmarkRenderer_Render_Checkboxes(t *testing.T) {
	// Create a renderer
	renderer := utils.GoldmarkRenderer{}

	// Define a markdown string with checkboxes
	source := []byte("- [x] Done\n- [ ] Not Done")

	// Call the Render method
	output, err := renderer.Render(source)

	// Check if there was an error
	if err != nil {
		t.Error(err)
	}

	// Define the expected HTML output
	expected := []byte("<ul>\n<li><input checked=\"\" disabled=\"\" type=\"checkbox\" /> Done</li>\n<li><input disabled=\"\" type=\"checkbox\" /> Not Done</li>\n</ul>\n")

	// Compare the expected output with the actual output
	if string(expected) != string(output) {
		t.Errorf("expected: %s, got: %s", expected, output)
	}
}

func TestGoldmarkRenderer_Render_Emojis(t *testing.T) {
	// Create a renderer
	renderer := utils.GoldmarkRenderer{}

	// Define a markdown string with an emoji
	source := []byte("I am so happy :joy:")

	// Call the Render method
	output, err := renderer.Render(source)

	// Check if there was an error
	if err != nil {
		t.Error(err)
	}

	// Define the expected HTML output
	expected := []byte("<p>I am so happy &#x1f602;</p>\n")

	// Compare the expected output with the actual output
	if string(expected) != string(output) {
		t.Errorf("expected: %s, got: %s", expected, output)
	}
}
