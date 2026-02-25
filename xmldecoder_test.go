package goxml

import (
	"strings"
	"testing"
)

// smallXML has a few elements with attributes and text content.
const smallXML = `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns:ns="http://example.com/ns">
  <item id="1" ns:type="a">Hello</item>
  <item id="2" ns:type="b">World</item>
  <item id="3">Text &amp; more</item>
</root>`

// genLargeXML builds an XML string with n child elements, each having
// attributes and text content, to exercise the parser on realistic input.
func genLargeXML(n int) string {
	var sb strings.Builder
	sb.WriteString(`<root xmlns:ns="http://example.com/ns">`)
	for i := 0; i < n; i++ {
		sb.WriteString(`<item id="`)
		sb.WriteString(strings.Repeat("x", 5))
		sb.WriteString(`" ns:type="val" class="cls">Some text content here</item>`)
	}
	sb.WriteString(`</root>`)
	return sb.String()
}

var (
	largeXML = genLargeXML(1000)
)

func TestParseLatin1(t *testing.T) {
	// ISO-8859-1 encoded XML: byte 0xFC = ü, 0xE4 = ä
	raw := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n<root>Gr\xfc\xdfe und \xc4nderungen</root>")
	doc, err := Parse(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	root, _ := doc.Root()
	got := root.Stringvalue()
	want := "Grüße und Änderungen"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseLatin1Attributes(t *testing.T) {
	// Latin-1 bytes in attribute values: 0xE9 = é, 0xF6 = ö
	raw := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n<root name=\"caf\xe9\" city=\"K\xf6ln\" />")
	doc, err := Parse(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	root, _ := doc.Root()
	attrs := root.Attributes()
	want := map[string]string{
		"name": "café",
		"city": "Köln",
	}
	for _, a := range attrs {
		if w, ok := want[a.Name]; ok {
			if a.Value != w {
				t.Errorf("attr %s: got %q, want %q", a.Name, a.Value, w)
			}
			delete(want, a.Name)
		}
	}
	if len(want) > 0 {
		t.Errorf("missing attributes: %v", want)
	}
}

func TestParseLatin1HighBytes(t *testing.T) {
	// Test the full upper range (0x80–0xFF) survives the round-trip.
	// Build a Latin-1 text with bytes 0xA0..0xFF (printable upper half).
	var raw []byte
	raw = append(raw, []byte("<?xml version=\"1.0\" encoding=\"latin1\"?>\n<r>")...)
	for b := byte(0xA0); b != 0; b++ { // 0xA0..0xFF (wraps to 0)
		raw = append(raw, b)
	}
	raw = append(raw, []byte("</r>")...)

	doc, err := Parse(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	root, _ := doc.Root()
	got := root.Stringvalue()

	// Build the expected UTF-8 string from the same codepoints.
	var want strings.Builder
	for cp := rune(0xA0); cp <= 0xFF; cp++ {
		want.WriteRune(cp)
	}
	if got != want.String() {
		t.Errorf("high-byte round-trip failed:\n  got  %q\n  want %q", got, want.String())
	}
}

func TestParseLatin1RoundTrip(t *testing.T) {
	// Parse Latin-1, serialize to UTF-8 XML, re-parse, compare.
	raw := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n<doc><p lang=\"fran\xe7ais\">\xc0 bient\xf4t!</p></doc>")
	doc, err := Parse(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	// Serialize to UTF-8 XML string.
	xmlStr := doc.ToXML()

	// The output should contain the correct UTF-8 characters.
	if !strings.Contains(xmlStr, "français") {
		t.Errorf("expected 'français' in XML output, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "À bientôt!") {
		t.Errorf("expected 'À bientôt!' in XML output, got: %s", xmlStr)
	}

	// Re-parse the UTF-8 output.
	doc2, err := Parse(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	root2, _ := doc2.Root()
	if got := root2.Stringvalue(); got != "À bientôt!" {
		t.Errorf("round-trip text: got %q, want %q", got, "À bientôt!")
	}
}

func BenchmarkParse_Small(b *testing.B) {
	data := smallXML
	b.ResetTimer()
	for b.Loop() {
		_, _ = Parse(strings.NewReader(data))
	}
}

func BenchmarkParse_Large(b *testing.B) {
	data := largeXML
	b.ResetTimer()
	for b.Loop() {
		_, _ = Parse(strings.NewReader(data))
	}
}

func BenchmarkToXML_Small(b *testing.B) {
	doc, _ := Parse(strings.NewReader(smallXML))
	b.ResetTimer()
	for b.Loop() {
		_ = doc.ToXML()
	}
}

func BenchmarkToXML_Large(b *testing.B) {
	doc, _ := Parse(strings.NewReader(largeXML))
	b.ResetTimer()
	for b.Loop() {
		_ = doc.ToXML()
	}
}

func BenchmarkStringvalue_Small(b *testing.B) {
	doc, _ := Parse(strings.NewReader(smallXML))
	root, _ := doc.Root()
	b.ResetTimer()
	for b.Loop() {
		_ = root.Stringvalue()
	}
}

func BenchmarkStringvalue_Large(b *testing.B) {
	doc, _ := Parse(strings.NewReader(largeXML))
	root, _ := doc.Root()
	b.ResetTimer()
	for b.Loop() {
		_ = root.Stringvalue()
	}
}

func BenchmarkAttributes(b *testing.B) {
	doc, _ := Parse(strings.NewReader(largeXML))
	root, _ := doc.Root()
	items := root.Children()
	b.ResetTimer()
	for b.Loop() {
		for _, child := range items {
			if elt, ok := child.(*Element); ok {
				_ = elt.Attributes()
			}
		}
	}
}
