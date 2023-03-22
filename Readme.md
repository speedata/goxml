[![Go reference documentation](https://img.shields.io/badge/doc-go%20reference-73FA79)](https://pkg.go.dev/github.com/speedata/goxml)

# Go XML

Go XML is a DOM based XML representation for Go. The entire XML
file is read into a set of structs and can be accessed without keeping the
source file open.

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



License: BSD-3-Clause License