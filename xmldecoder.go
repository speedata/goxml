package goxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

var (
	entitiesReplacer *strings.Replacer
)

func init() {
	entitiesReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", "\"", "&quot;")
}

// XMLNode is one of Document, Element, CharData, ProcInst, Comment
type XMLNode interface {
	toxml(map[string]bool) string
	setParent(XMLNode)
	Children() []XMLNode
}

// Appender implements the function Append(XMLNode)
type Appender interface {
	Append(n XMLNode)
}

// Attribute represents an attribute
type Attribute struct {
	Name      string
	Namespace string
	Prefix    string
	Value     string
}

func (a Attribute) String() string {
	return fmt.Sprintf("%s=%q", a.Name, a.Value)
}

// Stringvalue returns the attribute value.
func (a Attribute) Stringvalue() string {
	return fmt.Sprintf("%s", a.Value)
}

// Element represents an XML element
type Element struct {
	Name       string
	Prefix     string
	Parent     XMLNode
	Namespaces map[string]string
	children   []XMLNode
	attributes []xml.Attr
}

// NewElement returns an initialized Element.
func NewElement() *Element {
	elt := Element{}
	elt.Namespaces = make(map[string]string)
	return &elt
}

func (elt Element) String() string {
	var as []string
	for _, attribs := range elt.Attributes() {
		as = append(as, attribs.String())
	}
	return "<" + elt.Name + " " + strings.Join(as, " ") + ">"
}

// Stringvalue returns the text nodes of this elements and its children.
func (elt Element) Stringvalue() string {
	var as []string
	for _, cld := range elt.children {
		switch t := cld.(type) {
		case CharData:
			as = append(as, string(t))
		case *Element:
			as = append(as, t.Stringvalue())
		}
	}
	return strings.Join(as, "")
}

// Append appends an XML node to the element.
func (elt *Element) Append(n XMLNode) {
	// combine string cdata string if necessary
	if cd, ok := n.(CharData); ok {
		if l := len(elt.children); l > 0 {
			if str, ok := elt.children[l-1].(CharData); ok {
				elt.children[l-1] = CharData(string(str) + string(cd))
				return
			}
		}
	}
	elt.children = append(elt.children, n)
}

// Children returns all child nodes from elt
func (elt *Element) Children() []XMLNode {
	return elt.children
}

// Attributes returns all attributes for this element
func (elt *Element) Attributes() []*Attribute {
	var attribs []*Attribute
	for _, xmlattr := range elt.attributes {
		attr := Attribute{}
		attr.Name = xmlattr.Name.Local
		attr.Value = xmlattr.Value
		attr.Namespace = xmlattr.Name.Space
		attribs = append(attribs, &attr)
	}
	return attribs
}

func (elt *Element) setParent(n XMLNode) {
	elt.Parent = n
}

// ToXML returns a valid XML document
func (elt *Element) ToXML() string {
	return elt.toxml(make(map[string]bool))
}

func (elt *Element) toxml(namespacePrinted map[string]bool) string {
	var sb strings.Builder
	sb.WriteRune('<')
	eltname := elt.Name
	if elt.Prefix != "" {
		eltname = elt.Prefix + ":" + eltname
	}
	sb.WriteString(eltname)

	for prefix, ns := range elt.Namespaces {
		if _, ok := namespacePrinted[ns]; !ok {
			namespacePrinted[ns] = true
			if prefix == "" {
				prefix = "xmlns"
			} else {
				prefix = "xmlns:" + prefix
			}
			fmt.Fprintf(&sb, " %s=\"%s\"", prefix, ns)
		}
	}

	for _, att := range elt.attributes {
		fmt.Fprintf(&sb, " %s=\"%s\"", att.Name.Local, escape(att.Value))
	}
	if len(elt.children) == 0 {
		sb.WriteString(" />")
		return sb.String()
	}
	sb.WriteString(">")
	for _, child := range elt.children {
		sb.WriteString(child.toxml(namespacePrinted))
	}
	fmt.Fprintf(&sb, "</%s>", eltname)
	return sb.String()
}

// CharData is a string
type CharData string

// toxml returns the XML representation of the string.
func (cd CharData) toxml(namespacePrinted map[string]bool) string {
	return escape(string(cd))
}

func (cd CharData) setParent(n XMLNode) {
	// dummy
}

// Children is a dummy function
func (cd CharData) Children() []XMLNode {
	return nil
}

// Comment is a string
type Comment string

// toxml returns the XML representation of the comment.
func (cmt Comment) toxml(namespacePrinted map[string]bool) string {
	return fmt.Sprintf("<!--%s-->", string(cmt))
}

func (cmt Comment) setParent(n XMLNode) {
	// dummy
}

// Children is a dummy function
func (cmt Comment) Children() []XMLNode {
	return nil
}

// ProcInst is a string
type ProcInst struct {
	Target string
	Inst   []byte
}

// toxml returns the XML representation of the string.
func (pi ProcInst) toxml(namespacePrinted map[string]bool) string {
	return fmt.Sprintf("<?%s %s?>", pi.Target, string(pi.Inst))
}

func (pi ProcInst) setParent(n XMLNode) {
	// dummy
}

// Children is a dummy function
func (pi ProcInst) Children() []XMLNode {
	return nil
}

// XMLDocument represents an XML file for decoding
type XMLDocument struct {
	children []XMLNode
}

func (xr XMLDocument) String() string {
	return "<xmldoc>"
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
	doc := &XMLDocument{}
	eltstack := []XMLNode{doc}
	cur = doc
	dec := xml.NewDecoder(r)

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
			if c, ok := cur.(*Element); ok {
				for k, v := range c.Namespaces {
					tmp.Namespaces[k] = v
				}
			}
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
			cd := CharData(string(v))
			if c, ok := cur.(Appender); ok {
				c.Append(cd)
			}
		case xml.ProcInst:
			pi := ProcInst{}
			pi.Target = v.Copy().Target
			pi.Inst = v.Copy().Inst
			if c, ok := cur.(Appender); ok {
				c.Append(pi)
			}
		case xml.Comment:
			cmt := Comment(string(v))
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
