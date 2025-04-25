package goxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

var (
	entitiesReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", "\"", "&quot;")
	ids              chan int
)

func genIntegerSequence(ids chan int) {
	i := int(0)
	for {
		ids <- i
		i++
	}
}

func init() {
	ids = make(chan int)
	go genIntegerSequence(ids)
}

// XMLNode is one of Document, Element, CharData, ProcInst, Comment
type XMLNode interface {
	toxml(map[string]bool) string
	setParent(XMLNode)
	getID() int
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
	panic("Attribute setParent: nyi")
}

// getID returns the ID of this node
func (a Attribute) getID() int {
	return a.ID
}

// toxml returns the XML representation of the attribute.
func (a Attribute) toxml(namespacePrinted map[string]bool) string {
	return fmt.Sprintf("%s=\"%s\"", a.Name, escape(a.Value))
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
	var as []string
	for _, cld := range elt.children {
		switch t := cld.(type) {
		case CharData:
			as = append(as, string(t.Contents))
		case *Element:
			as = append(as, t.Stringvalue())
		}
	}
	return strings.Join(as, "")
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
		for k, v := range elt.Namespaces {
			if v == attr.Name.Space {
				attrPrefix = k
				break
			}
		}
		attr.Name.Local = attrPrefix + ":" + attr.Name.Local
	}
	newAttributes = append(newAttributes, attr)
	elt.attributes = newAttributes
}

// Attributes returns all attributes for this element
func (elt Element) Attributes() []*Attribute {
	var attribs []*Attribute
	for _, xmlattr := range elt.attributes {
		attr := Attribute{}
		attr.Name = xmlattr.Name.Local
		attr.Value = xmlattr.Value
		attr.Namespace = xmlattr.Name.Space
		for k, v := range elt.Namespaces {
			if v == xmlattr.Name.Space {
				attr.Prefix = k
			}
		}
		attribs = append(attribs, &attr)
	}
	return attribs
}

func (elt *Element) setParent(n XMLNode) {
	elt.Parent = n
}

// getID returns the ID of this node
func (elt *Element) getID() int {
	return elt.ID
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

// Comment is a string
type Comment struct {
	ID       int
	Contents string
}

// toxml returns the XML representation of the comment.
func (cmt Comment) toxml(namespacePrinted map[string]bool) string {
	return fmt.Sprintf("<!--%s-->", cmt.Contents)
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

// ProcInst is a string
type ProcInst struct {
	ID     int
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

// getID returns the ID of this node
func (pi ProcInst) getID() int {
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
	doc := &XMLDocument{ID: <-ids}
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
			tmp.ID = <-ids
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
			cd := CharData{ID: <-ids, Contents: string(v)}
			if c, ok := cur.(Appender); ok {
				c.Append(cd)
			}
		case xml.ProcInst:
			pi := ProcInst{ID: <-ids}
			pi.Target = v.Copy().Target
			pi.Inst = v.Copy().Inst
			if c, ok := cur.(Appender); ok {
				c.Append(pi)
			}
		case xml.Comment:
			cmt := Comment{ID: <-ids, Contents: string(v)}
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
