package goxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync/atomic"
)

var (
	entitiesReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", "\"", "&quot;")
	nextID           atomic.Int64
)

// charsetReader returns a reader that converts the named charset to UTF-8.
// Supports ISO-8859-1 (Latin-1) and US-ASCII without external dependencies.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "iso-8859-1", "latin-1", "latin1":
		return newLatin1Reader(input), nil
	case "us-ascii", "ascii":
		return input, nil
	case "utf-8":
		return input, nil
	default:
		return nil, fmt.Errorf("unsupported charset: %s", charset)
	}
}

// latin1Reader converts ISO-8859-1 encoded bytes to UTF-8.
type latin1Reader struct {
	r   io.Reader
	buf [1024]byte
}

func newLatin1Reader(r io.Reader) *latin1Reader {
	return &latin1Reader{r: r}
}

func (l *latin1Reader) Read(p []byte) (int, error) {
	// Read raw bytes, then expand each byte >= 0x80 to its 2-byte UTF-8 form.
	// We read at most half of len(p) raw bytes to guarantee enough room.
	max := len(p) / 2
	if max == 0 {
		max = 1
	}
	if max > len(l.buf) {
		max = len(l.buf)
	}
	n, err := l.r.Read(l.buf[:max])
	j := 0
	for i := 0; i < n; i++ {
		b := l.buf[i]
		if b < 0x80 {
			p[j] = b
			j++
		} else {
			p[j] = 0xC0 | (b >> 6)
			p[j+1] = 0x80 | (b & 0x3F)
			j += 2
		}
	}
	return j, err
}

func newID() int {
	return int(nextID.Add(1))
}

// XMLNode is one of Document, Element, CharData, ProcInst, Comment
type XMLNode interface {
	toxml(map[string]bool) string
	setParent(XMLNode)
	getID() int
	// GetID returns a unique integer identifier for this node.
	GetID() int
	Children() []XMLNode
}

// SortAndEliminateDuplicates returns the nodes sorted in document order and
// duplicates deleted.
func (xn SortByDocumentOrder) SortAndEliminateDuplicates() SortByDocumentOrder {
	sort.Sort(xn)
	if len(xn) < 2 {
		return xn
	}

	var e int = 1
	for i := 1; i < len(xn); i++ {
		if xn[i].getID() == xn[i-1].getID() {
			continue
		}
		xn[e] = xn[i]
		e++
	}

	return xn[:e]
}

// Appender implements the function Append(XMLNode)
type Appender interface {
	Append(n XMLNode)
}

// Attribute represents an attribute
type Attribute struct {
	ID        int
	Name      string
	Namespace string
	Prefix    string
	Value     string
	Parent    XMLNode
}

func (a Attribute) String() string {
	return fmt.Sprintf("%s=%q", a.Name, a.Value)
}

// Stringvalue returns the attribute value.
func (a Attribute) Stringvalue() string {
	return a.Value
}

// Children returns the empty sequence.
func (a Attribute) Children() []XMLNode {
	return nil
}

func (a Attribute) setParent(n XMLNode) {
	// no-op: Parent is set via Attributes() cache
}

// getID returns the ID of this node
func (a Attribute) getID() int {
	return a.ID
}

// GetID returns the ID of this node.
func (a Attribute) GetID() int {
	return a.ID
}

// toxml returns the XML representation of the attribute.
func (a Attribute) toxml(namespacePrinted map[string]bool) string {
	return a.Name + "=\"" + escape(a.Value) + "\""
}

// Element represents an XML element
type Element struct {
	ID         int
	Name       string
	Prefix     string
	Parent     XMLNode
	Namespaces map[string]string
	children   []XMLNode
	attributes []xml.Attr
	attrCache  []*Attribute // cached Attribute objects with stable IDs
	Line       int
	Pos        int
}

// NewElement returns an initialized Element.
func NewElement() *Element {
	elt := Element{}
	elt.Namespaces = make(map[string]string)
	return &elt
}

func (elt Element) String() string {
	return elt.Stringvalue()
}

// Stringvalue returns the text nodes of this elements and its children.
func (elt *Element) Stringvalue() string {
	var sb strings.Builder
	elt.appendStringvalue(&sb)
	return sb.String()
}

func (elt *Element) appendStringvalue(sb *strings.Builder) {
	for _, cld := range elt.children {
		switch t := cld.(type) {
		case CharData:
			sb.WriteString(t.Contents)
		case *Element:
			t.appendStringvalue(sb)
		}
	}
}

// Append appends an XML node to the element.
func (elt *Element) Append(n XMLNode) {
	switch t := n.(type) {
	case Attribute:
		for i, attr := range elt.attributes {
			if attr.Name.Local == t.Name && attr.Name.Space == t.Namespace {
				elt.attributes[i].Value = t.Value
				return
			}
		}
		elt.attributes = append(elt.attributes, xml.Attr{
			Name:  xml.Name{Local: t.Name, Space: t.Namespace},
			Value: t.Value,
		})
		return
	case *Attribute:
		for i, attr := range elt.attributes {
			if attr.Name.Local == t.Name && attr.Name.Space == t.Namespace {
				elt.attributes[i].Value = t.Value
				return
			}
		}
		elt.attributes = append(elt.attributes, xml.Attr{
			Name:  xml.Name{Local: t.Name, Space: t.Namespace},
			Value: t.Value,
		})
		return
	case CharData:
		// combine string cdata string if necessary
		if l := len(elt.children); l > 0 {
			if str, ok := elt.children[l-1].(CharData); ok {
				elt.children[l-1] = CharData{Contents: str.Contents + t.Contents}
				return
			}
		}
	case *CharData:
		// combine string cdata string if necessary
		if l := len(elt.children); l > 0 {
			if str, ok := elt.children[l-1].(*CharData); ok {
				elt.children[l-1] = CharData{Contents: str.Contents + t.Contents}
				return
			}
		}
	default:
		// do nothing
	}
	elt.children = append(elt.children, n)
}

// Children returns all child nodes from elt
func (elt *Element) Children() []XMLNode {
	return elt.children
}

// SetAttribute appends attr to the list of attributes of elt. If an attribute
// of this name already exists, the existing one will be discarded.
func (elt *Element) SetAttribute(attr xml.Attr) {
	var newAttributes = make([]xml.Attr, 0, len(elt.attributes)+1)
	// add all attributes except the one to be replaced
	for _, curattr := range elt.attributes {
		if curattr.Name != attr.Name {
			newAttributes = append(newAttributes, curattr)
		}
	}
	// add the new attribute
	if attr.Name.Space != "" {
		attrPrefix := ""
		if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" {
			attrPrefix = "xml"
		} else {
			for k, v := range elt.Namespaces {
				if v == attr.Name.Space {
					attrPrefix = k
					break
				}
			}
		}
		if attrPrefix != "" {
			attr.Name.Local = attrPrefix + ":" + attr.Name.Local
		}
	}
	newAttributes = append(newAttributes, attr)
	elt.attributes = newAttributes
	elt.attrCache = nil // invalidate cache
}

// Attributes returns all attributes for this element.
// Attribute objects are cached so they have stable IDs across calls.
func (elt *Element) Attributes() []*Attribute {
	if elt.attrCache != nil {
		return elt.attrCache
	}
	var attribs []*Attribute
	for _, xmlattr := range elt.attributes {
		attr := &Attribute{
			ID:        newID(),
			Name:      xmlattr.Name.Local,
			Value:     xmlattr.Value,
			Namespace: xmlattr.Name.Space,
			Parent:    elt,
		}
		for k, v := range elt.Namespaces {
			if v == xmlattr.Name.Space {
				attr.Prefix = k
			}
		}
		attribs = append(attribs, attr)
	}
	elt.attrCache = attribs
	return attribs
}

func (elt *Element) setParent(n XMLNode) {
	elt.Parent = n
}

// getID returns the ID of this node
func (elt *Element) getID() int {
	return elt.ID
}

// GetID returns the ID of this node.
func (elt *Element) GetID() int {
	return elt.ID
}

// ToXML returns a valid XML document
func (elt *Element) ToXML() string {
	return elt.toxml(make(map[string]bool))
}

// InnerXML returns the XML representation of the children of this element.
func (elt *Element) InnerXML() string {
	var sb strings.Builder
	for _, child := range elt.children {
		sb.WriteString(child.toxml(make(map[string]bool)))
	}
	return sb.String()
}

func (elt *Element) toxml(namespacePrinted map[string]bool) string {
	var sb strings.Builder
	sb.WriteRune('<')
	eltname := elt.Name
	if elt.Prefix != "" {
		eltname = elt.Prefix + ":" + eltname
	}
	sb.WriteString(eltname)

	// Sort namespace prefixes for deterministic output.
	nsPrefixes := make([]string, 0, len(elt.Namespaces))
	for prefix := range elt.Namespaces {
		nsPrefixes = append(nsPrefixes, prefix)
	}
	sort.Strings(nsPrefixes)
	for _, prefix := range nsPrefixes {
		ns := elt.Namespaces[prefix]
		if _, ok := namespacePrinted[ns]; !ok {
			namespacePrinted[ns] = true
			if prefix == "" {
				prefix = "xmlns"
			} else {
				prefix = "xmlns:" + prefix
			}
			sb.WriteByte(' ')
			sb.WriteString(prefix)
			sb.WriteString("=\"")
			sb.WriteString(ns)
			sb.WriteByte('"')
		}
	}

	for _, att := range elt.attributes {
		sb.WriteByte(' ')
		sb.WriteString(att.Name.Local)
		sb.WriteString("=\"")
		sb.WriteString(escape(att.Value))
		sb.WriteByte('"')
	}
	if len(elt.children) == 0 {
		sb.WriteString(" />")
		return sb.String()
	}
	sb.WriteString(">")
	for _, child := range elt.children {
		sb.WriteString(child.toxml(namespacePrinted))
	}
	sb.WriteString("</")
	sb.WriteString(eltname)
	sb.WriteByte('>')
	return sb.String()
}

// CharData is a string
type CharData struct {
	ID       int
	Contents string
}

// toxml returns the XML representation of the string.
func (cd CharData) toxml(namespacePrinted map[string]bool) string {
	return escape(string(cd.Contents))
}

func (cd CharData) setParent(n XMLNode) {
	// dummy
}

// Children is a dummy function
func (cd CharData) Children() []XMLNode {
	return nil
}

// getID returns the ID of this node
func (cd CharData) getID() int {
	return cd.ID
}

// GetID returns the ID of this node.
func (cd CharData) GetID() int {
	return cd.ID
}

// Comment is a string
type Comment struct {
	ID       int
	Contents string
}

// toxml returns the XML representation of the comment.
func (cmt Comment) toxml(namespacePrinted map[string]bool) string {
	return "<!--" + cmt.Contents + "-->"
}

func (cmt Comment) setParent(n XMLNode) {
	// dummy
}

// Children is a dummy function
func (cmt Comment) Children() []XMLNode {
	return nil
}

// getID returns the ID of this node
func (cmt Comment) getID() int {
	return cmt.ID
}

// GetID returns the ID of this node.
func (cmt Comment) GetID() int {
	return cmt.ID
}

// ProcInst is a string
type ProcInst struct {
	ID     int
	Target string
	Inst   []byte
}

// toxml returns the XML representation of the string.
func (pi ProcInst) toxml(namespacePrinted map[string]bool) string {
	return "<?" + pi.Target + " " + string(pi.Inst) + "?>"
}

func (pi ProcInst) setParent(n XMLNode) {
	// dummy
}

// Children is a dummy function
func (pi ProcInst) Children() []XMLNode {
	return nil
}

// getID returns the ID of this node
func (pi ProcInst) getID() int {
	return pi.ID
}

// GetID returns the ID of this node.
func (pi ProcInst) GetID() int {
	return pi.ID
}

// XMLDocument represents an XML file for decoding
type XMLDocument struct {
	ID       int
	children []XMLNode
}

func (xr XMLDocument) String() string {
	return "<xmldoc>"
}

// Stringvalue returns the string value of the document node (concatenation of
// all descendant text nodes), analogous to Element.Stringvalue().
func (xr *XMLDocument) Stringvalue() string {
	var sb strings.Builder
	for _, cld := range xr.children {
		switch t := cld.(type) {
		case CharData:
			sb.WriteString(t.Contents)
		case *Element:
			t.appendStringvalue(&sb)
		}
	}
	return sb.String()
}

// Append appends an XML node to the document.
func (xr *XMLDocument) Append(n XMLNode) {
	xr.children = append(xr.children, n)
	n.setParent(xr)
}

// Children returns all child nodes from elt
func (xr *XMLDocument) Children() []XMLNode {
	return xr.children
}

// Root returns the root node of the document
func (xr *XMLDocument) Root() (*Element, error) {
	for _, c := range xr.children {
		if elt, ok := c.(*Element); ok {
			return elt, nil
		}
	}
	return nil, fmt.Errorf("cannot find root element")
}

// ToXML returns a valid XML document
func (xr *XMLDocument) ToXML() string {
	return xr.toxml(make(map[string]bool))
}

func (xr *XMLDocument) setParent(n XMLNode) {
	// dummy,TODO error handling: document must not have a parent
}

// getID returns the ID of this node
func (xr XMLDocument) getID() int {
	return xr.ID
}

// GetID returns the ID of this node.
func (xr XMLDocument) GetID() int {
	return xr.ID
}

// toxml returns the XML representation of the document.
func (xr *XMLDocument) toxml(namespacePrinted map[string]bool) string {
	var sb strings.Builder
	for _, v := range xr.children {
		sb.WriteString(v.toxml(namespacePrinted))
	}
	return sb.String()
}

// Parse reads the XML file from r. r is not closed.
func Parse(r io.Reader) (*XMLDocument, error) {
	var err error
	var tok xml.Token

	var cur XMLNode
	doc := &XMLDocument{ID: newID()}
	eltstack := []XMLNode{doc}
	cur = doc
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charsetReader

	for {
		tok, err = dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			tmp := NewElement()
			tmp.ID = newID()
			if c, ok := cur.(*Element); ok {
				for k, v := range c.Namespaces {
					tmp.Namespaces[k] = v
				}
			}
			tmp.Line, tmp.Pos = dec.InputPos()
			tmp.Name = v.Name.Local
			tmp.Parent = cur

			for _, att := range v.Attr {
				if att.Name.Local == "xmlns" {
					tmp.Namespaces[""] = att.Value
				} else if att.Name.Space == "xmlns" {
					tmp.Namespaces[att.Name.Local] = att.Value
				} else {
					tmp.attributes = append(tmp.attributes, att)
				}
			}

			for prefix, ns := range tmp.Namespaces {
				if v.Name.Space == ns {
					tmp.Prefix = prefix
				}
			}

			if c, ok := cur.(Appender); ok {
				c.Append(tmp)
			}
			cur = tmp
			eltstack = append(eltstack, cur)
		case xml.CharData:
			// Skip whitespace-only text nodes at the document level —
			// only elements, PIs, and comments are allowed as document children.
			if _, isDoc := cur.(*XMLDocument); isDoc && strings.TrimSpace(string(v)) == "" {
				continue
			}
			cd := CharData{ID: newID(), Contents: string(v)}
			if c, ok := cur.(Appender); ok {
				c.Append(cd)
			}
		case xml.ProcInst:
			// The Go XML decoder returns the XML declaration as a ProcInst
			// with target "xml", but it is not a processing instruction.
			if v.Target == "xml" {
				continue
			}
			cp := v.Copy()
			pi := ProcInst{ID: newID(), Target: cp.Target, Inst: cp.Inst}
			if c, ok := cur.(Appender); ok {
				c.Append(pi)
			}
		case xml.Comment:
			cmt := Comment{ID: newID(), Contents: string(v)}
			if c, ok := cur.(Appender); ok {
				c.Append(cmt)
			}
		case xml.EndElement:
			cur, eltstack = eltstack[len(eltstack)-2], eltstack[:len(eltstack)-1]
		}
	}
	return doc, nil
}

func escape(in string) string {
	return entitiesReplacer.Replace(in)
}

// SortByDocumentOrder sorts the nodes by document order.
type SortByDocumentOrder []XMLNode

func (xn SortByDocumentOrder) Len() int      { return len(xn) }
func (xn SortByDocumentOrder) Swap(i, j int) { xn[i], xn[j] = xn[j], xn[i] }
func (xn SortByDocumentOrder) Less(i, j int) bool {
	return xn[i].getID() < xn[j].getID()
}
