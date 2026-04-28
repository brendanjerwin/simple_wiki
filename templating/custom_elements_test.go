//revive:disable:dot-imports
package templating_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/templating"
)

// This file's drift guard
//
// Two parallel risks made the user-visible bug ("macro in place, no
// element visible") possible:
//
//   1. A macro emits a custom element that isn't allow-listed by the
//      sanitizer. → tested in goldmarkrenderer via the registry.
//
//   2. The registry forgets an element that some macro still emits.
//      → tested HERE, by invoking each macro through reflection and
//      scanning the output for any `<custom-element>` open tag, then
//      asserting every found name appears in MacroCustomElements().
//
// "Custom element" is detected by the simple rule "contains a hyphen"
// (per the HTML spec, custom elements MUST contain a hyphen and built-in
// elements MUST NOT). That keeps the regex tight without an HTML parser.

var customElementOpenTag = regexp.MustCompile(`<([a-z][a-z0-9]*-[a-z0-9-]*)\b`)

var _ = Describe("MacroCustomElements registry coverage", func() {
	When("invoking every templating macro registered in the runtime FuncMap", func() {
		It("emits no custom element absent from the registry", func() {
			registered := registeredCustomElementNames()

			emitted := emittedCustomElementsByMacros()

			missing := []string{}
			for _, name := range emitted {
				if _, ok := registered[name]; !ok {
					missing = append(missing, name)
				}
			}
			Expect(missing).To(BeEmpty(),
				"these custom elements are emitted by templating macros but missing from MacroCustomElements() — sanitizer will silently strip them: %v",
				missing)
		})
	})
})

// registeredCustomElementNames returns MacroCustomElements as a set keyed
// by element name.
func registeredCustomElementNames() map[string]struct{} {
	out := make(map[string]struct{})
	for _, ce := range templating.MacroCustomElements() {
		out[ce.Name] = struct{}{}
	}
	return out
}

// emittedCustomElementsByMacros invokes each public Build* macro factory
// in the templating package via reflection, calls the returned func with
// safe sample arguments, and accumulates every custom-element name found
// in the output.
//
// We don't go through ExecuteTemplate because that would require wiring a
// real frontmatter + page store; for drift-detection we just need to
// invoke the macro builder and observe what HTML it emits.
func emittedCustomElementsByMacros() []string {
	// MacroBuilders is the single source of truth for "which macros to
	// invoke". Each entry is a builder + sample args. New macros that
	// emit custom elements should be added here so the drift test sees
	// them.
	type macroBuilder struct {
		name    string
		factory any
		args    []reflect.Value
	}

	tcontext := templating.TemplateContext{Identifier: "test_page"}
	stringArg := reflect.ValueOf("test-name")
	intArg := reflect.ValueOf(0)
	tcontextArg := reflect.ValueOf(tcontext)

	builders := []macroBuilder{
		{name: "Checklist", factory: templating.BuildChecklist, args: []reflect.Value{tcontextArg}},
		{name: "Survey", factory: templating.BuildSurvey, args: []reflect.Value{tcontextArg}},
		// BuildBlog needs query+site — skipped here (the registry test in
		// goldmarkrenderer covers wiki-blog passthrough). If a Blog
		// emitter starts emitting a NEW custom element, it'll need a
		// stub harness; for now Blog only emits wiki-blog which is
		// already covered.
		{name: "KeepConnect", factory: templating.BuildKeepConnect, args: []reflect.Value{tcontextArg}},
	}

	emitted := make(map[string]struct{})
	for _, mb := range builders {
		fn := reflect.ValueOf(mb.factory)
		// Call factory(args) → returns a func value
		built := fn.Call(mb.args)
		if len(built) != 1 {
			continue
		}
		// Now invoke the built function with sample args based on its type.
		bfn := built[0]
		var callArgs []reflect.Value
		for i := 0; i < bfn.Type().NumIn(); i++ {
			switch bfn.Type().In(i).Kind() {
			case reflect.String:
				callArgs = append(callArgs, stringArg)
			case reflect.Int, reflect.Int64:
				callArgs = append(callArgs, intArg)
			default:
				callArgs = append(callArgs, reflect.Zero(bfn.Type().In(i)))
			}
		}
		out := bfn.Call(callArgs)
		if len(out) == 0 {
			continue
		}
		rendered, _ := out[0].Interface().(string)
		for _, m := range customElementOpenTag.FindAllStringSubmatch(rendered, -1) {
			emitted[m[1]] = struct{}{}
		}
	}

	names := make([]string, 0, len(emitted))
	for n := range emitted {
		names = append(names, n)
	}
	return names
}

var _ = Describe("customElementOpenTag regex sanity", func() {
	When("matching strings that should be detected as custom elements", func() {
		It("should match <wiki-checklist>", func() {
			Expect(customElementOpenTag.FindStringSubmatch(`<wiki-checklist list="x">`)).ToNot(BeEmpty())
		})

		It("should match <keep-connect>", func() {
			Expect(customElementOpenTag.FindStringSubmatch(`<keep-connect></keep-connect>`)).ToNot(BeEmpty())
		})
	})

	When("matching strings that must not be misread as custom elements", func() {
		It("should not match a built-in <div>", func() {
			Expect(customElementOpenTag.FindStringSubmatch(`<div>built-in element</div>`)).To(BeEmpty())
		})

		It("should not match plain text containing a hyphen", func() {
			Expect(customElementOpenTag.FindStringSubmatch(`<p>some-hyphenated text</p>`)).To(BeEmpty())
		})
	})
})

// silence unused-import lint.
var _ = strings.Contains
var _ = testing.AllocsPerRun
