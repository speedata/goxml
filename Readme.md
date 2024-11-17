[![Go reference documentation](https://img.shields.io/badge/doc-go%20reference-73FA79)](https://pkg.go.dev/github.com/speedata/goxml)

# Go XML

Go XML is a DOM based XML representation for Go. The entire XML
file is read into a set of structs and can be accessed without keeping the
source file open. You can also use this library to construct and serialize an XML file.

Used in https://github.com/speedata/goxpath

## Sample usage

`myfile.xml`:

```xml
<data attrib="hello">
    <p>hello world!</p>
</data>
```

`main.go`:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/speedata/goxml"
)

func dothings() error {
	r, err := os.Open("myfile.xml")
	if err != nil {
		return err
	}
	doc, err := goxml.Parse(r)
	if err != nil {
		return err
	}
	rootElt, err := doc.Root()
	if err != nil {
		return err
	}
	fmt.Println(rootElt.Attributes()) // [attrib="hello"]
	_ = rootElt.Parent                // document node

	return nil
}

func main() {
	if err := dothings(); err != nil {
		log.Fatal(err)
	}
}
```

Constructing an XML file:

~~~go
package main

import (
	"encoding/xml"
	"fmt"

	"github.com/speedata/goxml"
)

func main() {
	d := goxml.XMLDocument{}
	root := &goxml.Element{Name: "root"}
	d.Append(root)
	cd := goxml.CharData{Contents: "\n   "}
	root.Append(cd)
	elt1 := &goxml.Element{Name: "element"}
	elt1.SetAttribute(xml.Attr{Name: xml.Name{Local: "attr"}, Value: "element 1"})
	elt1.SetAttribute(xml.Attr{Name: xml.Name{Local: "attr2"}, Value: "some <value> &'"})
	root.Append(elt1)
	root.Append(cd)
	elt2 := &goxml.Element{Name: "element"}
	elt2.SetAttribute(xml.Attr{Name: xml.Name{Local: "attr"}, Value: "element 2"})
	root.Append(elt2)
	root.Append(goxml.CharData{Contents: "\n"})
	fmt.Println(d.ToXML())
}
~~~

prints

```xml
<root>
   <element attr="element 1" attr2="some &lt;value> &amp;'" />
   <element attr="element 2" />
</root>
```


License: BSD-3-Clause License