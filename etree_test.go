// Copyright 2015-2019 Brett Vickers.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package etree

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path"
	"strings"
	"testing"
)

func newDocumentFromString(t *testing.T, s string) *Document {
	return newDocumentFromString2(t, s, ReadSettings{})
}

func newDocumentFromString2(t *testing.T, s string, settings ReadSettings) *Document {
	t.Helper()
	doc := NewDocument()
	doc.ReadSettings = settings
	err := doc.ReadFromString(s)
	if err != nil {
		t.Fatal("etree: failed to parse document")
	}
	return doc
}

func checkStrEq(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("etree: unexpected result.\nGot:\n%s\nWanted:\n%s\n", got, want)
	}
}

func checkStrBinaryEq(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("etree: unexpected result.\nGot:\n%v\nWanted:\n%v\n", []byte(got), []byte(want))
	}
}

func checkIntEq(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("etree: unexpected integer. Got: %d. Wanted: %d\n", got, want)
	}
}

func checkBoolEq(t *testing.T, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("etree: unexpected boolean. Got: %v. Wanted: %v\n", got, want)
	}
}

func checkElementEq(t *testing.T, got, want *Element) {
	t.Helper()
	if got != want {
		t.Errorf("etree: unexpected element. Got: %v. Wanted: %v.\n", got, want)
	}
}

func checkDocEq(t *testing.T, doc *Document, expected string) {
	t.Helper()
	doc.Indent(NoIndent)
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}
	if s != expected {
		t.Errorf("etree: unexpected document.\nGot:\n%s\nWanted:\n%s\n", s, expected)
	}
}

func checkIndexes(t *testing.T, e *Element) {
	t.Helper()
	for i := 0; i < len(e.Child); i++ {
		c := e.Child[i]
		if c.Index() != i {
			t.Errorf("Child index mismatch. Got %d, expected %d.", c.Index(), i)
		}
		if ce, ok := c.(*Element); ok {
			checkIndexes(t, ce)
		}
	}
}

func TestDocument(t *testing.T) {
	// Create a document
	doc := NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
	doc.CreateProcInst("xml-stylesheet", `type="text/xsl" href="style.xsl"`)
	store := doc.CreateElement("store")
	store.CreateAttr("xmlns:t", "urn:books-com:titles")
	store.CreateDirective("Directive")
	store.CreateComment("This is a comment")
	book := store.CreateElement("book")
	book.CreateAttr("lang", "fr")
	book.CreateAttr("lang", "en")
	title := book.CreateElement("t:title")
	title.SetText("Nicholas Nickleby")
	title.SetText("Great Expectations")
	author := book.CreateElement("author")
	author.CreateCharData("Charles Dickens")
	review := book.CreateElement("review")
	review.CreateCData("<<< Will be replaced")
	review.SetCData(">>> Excellent book")
	doc.IndentTabs()

	checkIndexes(t, &doc.Element)

	// Serialize the document to a string
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}

	// Make sure the serialized XML matches expectation.
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="style.xsl"?>
<store xmlns:t="urn:books-com:titles">
	<!Directive>
	<!--This is a comment-->
	<book lang="en">
		<t:title>Great Expectations</t:title>
		<author>Charles Dickens</author>
		<review><![CDATA[>>> Excellent book]]></review>
	</book>
</store>
`
	checkStrEq(t, s, expected)

	// Test the structure of the XML
	if doc.Root() != store {
		t.Error("etree: root mismatch")
	}
	if len(store.ChildElements()) != 1 || len(store.Child) != 7 {
		t.Error("etree: incorrect tree structure")
	}
	if len(book.ChildElements()) != 3 || len(book.Attr) != 1 || len(book.Child) != 7 {
		t.Error("etree: incorrect tree structure")
	}
	if len(title.ChildElements()) != 0 || len(title.Child) != 1 || len(title.Attr) != 0 {
		t.Error("etree: incorrect tree structure")
	}
	if len(author.ChildElements()) != 0 || len(author.Child) != 1 || len(author.Attr) != 0 {
		t.Error("etree: incorrect tree structure")
	}
	if len(review.ChildElements()) != 0 || len(review.Child) != 1 || len(review.Attr) != 0 {
		t.Error("etree: incorrect tree structure")
	}
	if book.parent != store || store.parent != &doc.Element || doc.parent != nil {
		t.Error("etree: incorrect tree structure")
	}
	if title.parent != book || author.parent != book {
		t.Error("etree: incorrect tree structure")
	}

	// Perform some basic queries on the document
	elements := doc.SelectElements("store")
	if len(elements) != 1 || elements[0] != store {
		t.Error("etree: incorrect SelectElements result")
	}
	element := doc.SelectElement("store")
	if element != store {
		t.Error("etree: incorrect SelectElement result")
	}
	elements = store.SelectElements("book")
	if len(elements) != 1 || elements[0] != book {
		t.Error("etree: incorrect SelectElements result")
	}
	element = store.SelectElement("book")
	if element != book {
		t.Error("etree: incorrect SelectElement result")
	}
	attr := book.SelectAttr("lang")
	if attr == nil || attr.Key != "lang" || attr.Value != "en" {
		t.Error("etree: incorrect SelectAttr result")
	}
	if book.SelectAttrValue("lang", "unknown") != "en" {
		t.Error("etree: incorrect SelectAttrValue result")
	}
	if book.SelectAttrValue("t:missing", "unknown") != "unknown" {
		t.Error("etree: incorrect SelectAttrValue result")
	}
	attr = book.RemoveAttr("lang")
	if attr.Value != "en" {
		t.Error("etree: incorrect RemoveAttr result")
	}
	book.CreateAttr("lang", "de")
	attr = book.RemoveAttr("lang")
	if attr.Value != "de" {
		t.Error("etree: incorrect RemoveAttr result")
	}
	element = book.SelectElement("t:title")
	if element != title || element.Text() != "Great Expectations" || len(element.Attr) != 0 {
		t.Error("etree: incorrect SelectElement result")
	}
	element = book.SelectElement("title")
	if element != title {
		t.Error("etree: incorrect SelectElement result")
	}
	element = book.SelectElement("p:title")
	if element != nil {
		t.Error("etree: incorrect SelectElement result")
	}
	element = book.RemoveChildAt(title.Index()).(*Element)
	if element != title {
		t.Error("etree: incorrect RemoveElement result")
	}
	element = book.SelectElement("title")
	if element != nil {
		t.Error("etree: incorrect SelectElement result")
	}
	element = book.SelectElement("review")
	if element != review || element.Text() != ">>> Excellent book" || len(element.Attr) != 0 {
		t.Error("etree: incorrect SelectElement result")
	}
}

func TestImbalancedXML(t *testing.T) {
	cases := []string{
		`<test>`,
		`</test>`,
		`<test></test2>`,
		`<doc xmlns:p="xyz"><p:test></test></doc>`,
		`<doc xmlns:p="xyz"><test></p:test></doc>`,
		`<test>malformed`,
		`malformed</test>`,
		`<test><test></test>`,
		`<test></test></test>`,
		`<test><test></test></test2>`,
		`<test><test2></test></test2>`,
	}
	for _, c := range cases {
		doc := NewDocument()
		err := doc.ReadFromString(c)
		if err == nil {
			t.Errorf("etree: imbalanced XML should have failed:\n%s", c)
		}
	}
}

func TestDocumentCharsetReader(t *testing.T) {
	s := `<?xml version="1.0" encoding="lowercase"?>
<Store>
	<Book Lang="en">
		<Title>Great Expectations</Title>
		<Author>Charles Dickens</Author>
	</Book>
</Store>`

	doc := newDocumentFromString2(t, s, ReadSettings{
		CharsetReader: func(label string, input io.Reader) (io.Reader, error) {
			if label == "lowercase" {
				return &lowercaseCharsetReader{input}, nil
			}
			return nil, errors.New("unknown charset")
		},
	})

	cases := []struct {
		path string
		text string
	}{
		{"/store/book/title", "great expectations"},
		{"/store/book/author", "charles dickens"},
	}
	for _, c := range cases {
		e := doc.FindElement(c.path)
		if e == nil {
			t.Errorf("etree: failed to find element '%s'", c.path)
		} else if e.Text() != c.text {
			t.Errorf("etree: expected path '%s' to contain '%s', got '%s'", c.path, c.text, e.Text())
		}
	}
}

type lowercaseCharsetReader struct {
	r io.Reader
}

func (c *lowercaseCharsetReader) Read(p []byte) (n int, err error) {
	n, err = c.r.Read(p)
	if err != nil {
		return n, err
	}
	for i := 0; i < n; i++ {
		if p[i] >= 'A' && p[i] <= 'Z' {
			p[i] = p[i] - 'A' + 'a'
		}
	}
	return n, nil
}

func TestDocumentReadPermissive(t *testing.T) {
	s := "<select disabled></select>"

	doc := NewDocument()
	err := doc.ReadFromString(s)
	if err == nil {
		t.Fatal("etree: incorrect ReadFromString result")
	}

	doc.ReadSettings.Permissive = true
	err = doc.ReadFromString(s)
	if err != nil {
		t.Fatal("etree: incorrect ReadFromString result")
	}
}

func TestEmbeddedComment(t *testing.T) {
	s := `<a>123<!-- test -->456</a>`

	doc := NewDocument()
	err := doc.ReadFromString(s)
	if err != nil {
		t.Fatal("etree: incorrect ReadFromString result")
	}

	a := doc.SelectElement("a")
	checkStrEq(t, a.Text(), "123456")
}

func TestDocumentReadHTMLEntities(t *testing.T) {
	s := `<store>
	<book lang="en">
		<title>&rarr;&nbsp;Great Expectations</title>
		<author>Charles Dickens</author>
	</book>
</store>`

	doc := NewDocument()
	err := doc.ReadFromString(s)
	if err == nil {
		t.Fatal("etree: incorrect ReadFromString result")
	}

	doc.ReadSettings.Entity = xml.HTMLEntity
	err = doc.ReadFromString(s)
	if err != nil {
		t.Fatal("etree: incorrect ReadFromString result")
	}
}

func TestDocumentReadHTMLAutoClose(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", ``, ``},
		{"oneSelfClosing", `<br>`, `<br/>`},
		{"twoSelfClosingAndText", `<br>some text<br>`, `<br/>some text<br/>`},
		{
			name: "largerExample",
			input: `<img src="cover.jpg">
<hr>
Author: Charles Dickens<br>
Book: Great Expectations<br>`,
			want: `<img src="cover.jpg"/>
<hr/>
Author: Charles Dickens<br/>
Book: Great Expectations<br/>`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := NewDocument()
			doc.ReadSettings.Permissive = true
			doc.ReadSettings.AutoClose = xml.HTMLAutoClose
			err := doc.ReadFromString(c.input)
			if err != nil {
				t.Fatal("etree: ReadFromString() error = ", err)
			}
			s, err := doc.WriteToString()
			if err != nil {
				t.Fatal("etree: WriteToString() error = ", err)
			}
			checkStrEq(t, s, c.want)
		})
	}
}

func TestEscapeCodes(t *testing.T) {
	cases := []struct {
		input         string
		normal        string
		attrCanonical string
		textCanonical string
	}{
		{
			"&<>'\"\t\n\r",
			"<e a=\"&amp;&lt;&gt;&apos;&quot;\t\n\r\">&amp;&lt;&gt;&apos;&quot;\t\n\r</e>",
			"<e a=\"&amp;&lt;>'&quot;&#x9;&#xA;&#xD;\">&amp;&lt;&gt;&apos;&quot;\t\n\r</e>",
			"<e a=\"&amp;&lt;&gt;&apos;&quot;\t\n\r\">&amp;&lt;&gt;'\"\t\n&#xD;</e>",
		},
		{
			"\x00\x1f\x08\x09\x0a\x0d",
			"<e a=\"���\t\n\r\">���\t\n\r</e>",
			"<e a=\"���&#x9;&#xA;&#xD;\">���\t\n\r</e>",
			"<e a=\"���\t\n\r\">���\t\n&#xD;</e>",
		},
	}
	for _, c := range cases {
		doc := NewDocument()

		e := doc.CreateElement("e")
		e.SetText(c.input)
		e.CreateAttr("a", c.input)

		doc.WriteSettings.CanonicalText = false
		doc.WriteSettings.CanonicalAttrVal = false
		s, err := doc.WriteToString()
		if err != nil {
			t.Error("etree: Escape test produced inocrrect result.")
		}
		checkStrEq(t, s, c.normal)

		doc.WriteSettings.CanonicalText = false
		doc.WriteSettings.CanonicalAttrVal = true
		s, err = doc.WriteToString()
		if err != nil {
			t.Error("etree: Escape test produced inocrrect result.")
		}
		checkStrEq(t, s, c.attrCanonical)

		doc.WriteSettings.CanonicalText = true
		doc.WriteSettings.CanonicalAttrVal = false
		s, err = doc.WriteToString()
		if err != nil {
			t.Error("etree: Escape test produced inocrrect result.")
		}
		checkStrEq(t, s, c.textCanonical)
	}
}

func TestCanonical(t *testing.T) {
	BOM := "\xef\xbb\xbf"

	doc := NewDocument()
	doc.WriteSettings.CanonicalEndTags = true
	doc.WriteSettings.CanonicalText = true
	doc.WriteSettings.CanonicalAttrVal = true
	doc.CreateCharData(BOM)
	doc.CreateProcInst("xml-stylesheet", `type="text/xsl" href="style.xsl"`)

	people := doc.CreateElement("People")
	people.CreateComment("These are all known people")

	jon := people.CreateElement("Person")
	jon.CreateAttr("name", "Jon O'Reilly")
	jon.SetText("\r<'\">&\u0004\u0005\u001f�")

	sally := people.CreateElement("Person")
	sally.CreateAttr("name", "Sally")
	sally.CreateAttr("escape", "\r\n\t<'\">&")

	doc.Indent(2)
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: WriteSettings WriteTo produced incorrect result.")
	}

	expected := BOM + `<?xml-stylesheet type="text/xsl" href="style.xsl"?>
<People>
  <!--These are all known people-->
  <Person name="Jon O'Reilly">&#xD;&lt;'"&gt;&amp;����</Person>
  <Person name="Sally" escape="&#xD;&#xA;&#x9;&lt;'&quot;>&amp;"></Person>
</People>
`
	checkStrEq(t, s, expected)
}

func TestCopy(t *testing.T) {
	s := `<store>
	<book lang="en">
		<title>Great Expectations</title>
		<author>Charles Dickens</author>
	</book>
</store>`

	doc := newDocumentFromString(t, s)

	s1, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: incorrect WriteToString result")
	}

	doc2 := doc.Copy()
	checkIndexes(t, &doc2.Element)
	s2, err := doc2.WriteToString()
	if err != nil {
		t.Error("etree: incorrect Copy result")
	}

	if s1 != s2 {
		t.Error("etree: mismatched Copy result")
		t.Error("wanted:\n" + s1)
		t.Error("got:\n" + s2)
	}

	e1 := doc.FindElement("./store/book/title")
	e2 := doc2.FindElement("./store/book/title")
	if e1 == nil || e2 == nil || e1.parent == nil || e1 == e2 {
		t.Error("etree: incorrect FindElement result")
	}

	e1.parent.RemoveChildAt(e1.Index())
	s1, _ = doc.WriteToString()
	s2, _ = doc2.WriteToString()
	if s1 == s2 {
		t.Error("etree: incorrect result after RemoveElement")
	}
}

func TestGetPath(t *testing.T) {
	s := `<a>
 <b1>
  <c1>
   <d1/>
   <d1a/>
  </c1>
 </b1>
 <b2>
  <c2>
   <d2/>
  </c2>
 </b2>
</a>`

	doc := newDocumentFromString(t, s)

	cases := []struct {
		from    string
		to      string
		relpath string
		topath  string
	}{
		{"a", ".", "..", "/"},
		{".", "a", "./a", "/a"},
		{"a/b1/c1/d1", ".", "../../../..", "/"},
		{".", "a/b1/c1/d1", "./a/b1/c1/d1", "/a/b1/c1/d1"},
		{"a", "a", ".", "/a"},
		{"a/b1", "a/b1/c1", "./c1", "/a/b1/c1"},
		{"a/b1/c1", "a/b1", "..", "/a/b1"},
		{"a/b1/c1", "a/b1/c1", ".", "/a/b1/c1"},
		{"a", "a/b1", "./b1", "/a/b1"},
		{"a/b1", "a", "..", "/a"},
		{"a", "a/b1/c1", "./b1/c1", "/a/b1/c1"},
		{"a/b1/c1", "a", "../..", "/a"},
		{"a/b1/c1/d1", "a", "../../..", "/a"},
		{"a", "a/b1/c1/d1", "./b1/c1/d1", "/a/b1/c1/d1"},
		{"a/b1", "a/b2", "../b2", "/a/b2"},
		{"a/b2", "a/b1", "../b1", "/a/b1"},
		{"a/b1/c1/d1", "a/b2/c2/d2", "../../../b2/c2/d2", "/a/b2/c2/d2"},
		{"a/b2/c2/d2", "a/b1/c1/d1", "../../../b1/c1/d1", "/a/b1/c1/d1"},
		{"a/b1/c1/d1", "a/b1/c1/d1a", "../d1a", "/a/b1/c1/d1a"},
	}

	for _, c := range cases {
		fe := doc.FindElement(c.from)
		te := doc.FindElement(c.to)

		rp := te.GetRelativePath(fe)
		if rp != c.relpath {
			t.Errorf("GetRelativePath from '%s' to '%s'. Expected '%s', got '%s'.\n", c.from, c.to, c.relpath, rp)
		}

		p := te.GetPath()
		if p != c.topath {
			t.Errorf("GetPath for '%s'. Expected '%s', got '%s'.\n", c.to, c.topath, p)
		}
	}
}

func TestInsertChild(t *testing.T) {
	s := `<book lang="en">
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
</book>
`

	doc := newDocumentFromString(t, s)

	year := NewElement("year")
	year.SetText("1861")

	book := doc.FindElement("//book")
	book.InsertChildAt(book.SelectElement("t:title").Index(), year)

	expected1 := `<book lang="en">
  <year>1861</year>
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
</book>
`
	doc.Indent(2)
	s1, _ := doc.WriteToString()
	checkStrEq(t, s1, expected1)

	book.RemoveChildAt(year.Index())
	book.InsertChildAt(book.SelectElement("author").Index(), year)

	expected2 := `<book lang="en">
  <t:title>Great Expectations</t:title>
  <year>1861</year>
  <author>Charles Dickens</author>
</book>
`
	doc.Indent(2)
	s2, _ := doc.WriteToString()
	checkStrEq(t, s2, expected2)

	book.RemoveChildAt(year.Index())
	book.InsertChildAt(len(book.Child), year)

	expected3 := `<book lang="en">
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
  <year>1861</year>
</book>
`
	doc.Indent(2)
	s3, _ := doc.WriteToString()
	checkStrEq(t, s3, expected3)

	book.RemoveChildAt(year.Index())
	book.InsertChildAt(999, year)

	expected4 := `<book lang="en">
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
  <year>1861</year>
</book>
`
	doc.Indent(2)
	s4, _ := doc.WriteToString()
	checkStrEq(t, s4, expected4)

	year = doc.FindElement("//book/year")
	book.InsertChildAt(0, year)

	expected5 := `<book lang="en">
  <year>1861</year>
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
</book>
`

	doc.Indent(2)
	s5, _ := doc.WriteToString()
	checkStrEq(t, s5, expected5)

	author := doc.FindElement("//book/author")
	year = doc.FindElement("//book/year")
	book.InsertChildAt(author.Index(), year)

	expected6 := `<book lang="en">
  <t:title>Great Expectations</t:title>
  <year>1861</year>
  <author>Charles Dickens</author>
</book>
`
	doc.Indent(2)
	s6, _ := doc.WriteToString()
	checkStrEq(t, s6, expected6)
}

func TestCdata(t *testing.T) {
	var tests = []struct {
		in, out string
	}{
		{`<tag>1234567</tag>`, "1234567"},
		{`<tag><![CDATA[1234567]]></tag>`, "1234567"},
		{`<tag>1<![CDATA[2]]>3<![CDATA[4]]>5<![CDATA[6]]>7</tag>`, "1234567"},
		{`<tag>1<![CDATA[2]]>3<inner>4</inner>5<![CDATA[6]]>7</tag>`, "123"},
		{`<tag>1<inner>4</inner>5<![CDATA[6]]>7</tag>`, "1"},
		{`<tag><![CDATA[1]]><inner>4</inner>5<![CDATA[6]]>7</tag>`, "1"},
	}

	for _, test := range tests {
		doc := NewDocument()
		err := doc.ReadFromString(test.in)
		if err != nil {
			t.Fatal("etree ReadFromString: " + err.Error())
		}

		tag := doc.FindElement("tag")
		if tag.Text() != test.out {
			t.Fatalf("etree invalid cdata. Expected: %v. Got: %v\n", test.out, tag.Text())
		}
	}
}

func TestAddChild(t *testing.T) {
	s := `<book lang="en">
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
</book>
`
	doc1 := newDocumentFromString(t, s)

	doc2 := NewDocument()
	root := doc2.CreateElement("root")

	for _, e := range doc1.FindElements("//book/*") {
		root.AddChild(e)
	}

	expected1 := `<book lang="en"/>
`
	doc1.Indent(2)
	s1, _ := doc1.WriteToString()
	checkStrEq(t, s1, expected1)

	expected2 := `<root>
  <t:title>Great Expectations</t:title>
  <author>Charles Dickens</author>
</root>
`
	doc2.Indent(2)
	s2, _ := doc2.WriteToString()
	checkStrEq(t, s2, expected2)
}

func TestSetRoot(t *testing.T) {
	s := `<?test a="wow"?>
<book>
  <title>Great Expectations</title>
  <author>Charles Dickens</author>
</book>
`
	doc := newDocumentFromString(t, s)

	origroot := doc.Root()
	if origroot.Parent() != &doc.Element {
		t.Error("Root incorrect")
	}

	newroot := NewElement("root")
	doc.SetRoot(newroot)

	if doc.Root() != newroot {
		t.Error("doc.Root() != newroot")
	}
	if origroot.Parent() != nil {
		t.Error("origroot.Parent() != nil")
	}

	expected1 := `<?test a="wow"?>
<root/>
`
	doc.Indent(2)
	s1, _ := doc.WriteToString()
	checkStrEq(t, s1, expected1)

	doc.SetRoot(origroot)
	doc.Indent(2)
	expected2 := s
	s2, _ := doc.WriteToString()
	checkStrEq(t, s2, expected2)

	doc2 := NewDocument()
	doc2.CreateProcInst("test", `a="wow"`)
	doc2.SetRoot(NewElement("root"))
	doc2.Indent(2)
	expected3 := expected1
	s3, _ := doc2.WriteToString()
	checkStrEq(t, s3, expected3)

	doc2.SetRoot(doc.Root())
	doc2.Indent(2)
	expected4 := s
	s4, _ := doc2.WriteToString()
	checkStrEq(t, s4, expected4)

	expected5 := `<?test a="wow"?>
`
	doc.Indent(2)
	s5, _ := doc.WriteToString()
	checkStrEq(t, s5, expected5)
}

func TestSortAttrs(t *testing.T) {
	s := `<el foo='5' Foo='2' aaa='4' สวัสดี='7' AAA='1' a01='3' z='6' a:ZZZ='9' a:AAA='8'/>`
	doc := newDocumentFromString(t, s)
	doc.Root().SortAttrs()
	doc.Indent(2)
	out, _ := doc.WriteToString()
	checkStrEq(t, out, `<el AAA="1" Foo="2" a01="3" aaa="4" foo="5" z="6" สวัสดี="7" a:AAA="8" a:ZZZ="9"/>`+"\n")
}

func TestCharsetReaderDefaultSetting(t *testing.T) {
	// Test encodings where the default pass-through charset conversion
	// should work for common single-byte character encodings.
	cases := []string{
		`<?xml version="1.0"?><foo></foo>`,
		`<?xml version="1.0" encoding="ISO-8859-1"?><foo></foo>`,
		`<?xml version="1.0" encoding="Windows-1252"?><foo></foo>`,
		`<?xml version="1.0" encoding="UTF-8"?><foo></foo>`,
		`<?xml version="1.0" encoding="US-ASCII"?><foo></foo>`,
	}

	for _, c := range cases {
		doc := NewDocument()
		if err := doc.ReadFromBytes([]byte(c)); err != nil {
			t.Error(err)
		}
	}
}

func TestCharData(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateElement("root")
	root.CreateCharData("This ")
	root.CreateCData("is ")
	e1 := NewText("a ")
	e2 := NewCData("text ")
	root.AddChild(e1)
	root.AddChild(e2)
	root.CreateCharData("Element!!")

	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}

	checkStrEq(t, s, `<root>This <![CDATA[is ]]>a <![CDATA[text ]]>Element!!</root>`)

	// Check we can parse the output
	err = doc.ReadFromString(s)
	if err != nil {
		t.Fatal("etree: incorrect ReadFromString result")
	}
	if doc.Root().Text() != "This is a text Element!!" {
		t.Error("etree: invalid text")
	}
}

func TestIndentSimple(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateElement("root")
	ch1 := root.CreateElement("child1")
	ch1.CreateElement("child2")

	// First test Unindent.
	doc.Unindent()
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}
	expected := "<root><child1><child2/></child1></root>"
	checkStrEq(t, s, expected)

	// Now test Indent with NoIndent (which should produce the same result
	// as Unindent).
	doc.Indent(NoIndent)
	s, err = doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}
	checkStrEq(t, s, expected)

	// Run all indent test cases.
	tests := []struct {
		useTabs, useCRLF bool
		ws, nl           string
	}{
		{false, false, " ", "\n"},
		{false, true, " ", "\r\n"},
		{true, false, "\t", "\n"},
		{true, true, "\t", "\r\n"},
	}

	for _, test := range tests {
		doc.WriteSettings.UseCRLF = test.useCRLF
		if test.useTabs {
			doc.IndentTabs()
			s, err := doc.WriteToString()
			if err != nil {
				t.Error("etree: failed to serialize document")
			}
			tab := test.ws
			expected := "<root>" + test.nl + tab + "<child1>" + test.nl +
				tab + tab + "<child2/>" + test.nl + tab +
				"</child1>" + test.nl + "</root>" + test.nl
			checkStrEq(t, s, expected)
		} else {
			for i := 0; i < 256; i++ {
				doc.Indent(i)
				s, err := doc.WriteToString()
				if err != nil {
					t.Error("etree: failed to serialize document")
				}
				tab := strings.Repeat(test.ws, i)
				expected := "<root>" + test.nl + tab + "<child1>" + test.nl +
					tab + tab + "<child2/>" + test.nl + tab +
					"</child1>" + test.nl + "</root>" + test.nl
				checkStrEq(t, s, expected)
			}
		}
	}
}

func TestIndentWithDefaultSettings(t *testing.T) {
	input := `<root>
	<child1>
		<child2>    </child2>
	</child1>
</root>`

	doc := NewDocument()
	err := doc.ReadFromString(input)
	if err != nil {
		t.Error("etree: failed to read string")
	}

	settings := NewIndentSettings()
	doc.IndentWithSettings(settings)
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}
	expected := "<root>\n    <child1>\n        <child2/>\n    </child1>\n</root>\n"
	checkStrEq(t, s, expected)
}

func TestIndentWithSettings(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateElement("root")
	ch1 := root.CreateElement("child1")
	ch1.CreateElement("child2")

	// First test with NoIndent.
	settings := NewIndentSettings()
	settings.UseCRLF = false
	settings.UseTabs = false
	settings.Spaces = NoIndent
	doc.IndentWithSettings(settings)
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}
	expected := "<root><child1><child2/></child1></root>"
	checkStrEq(t, s, expected)

	// Run all indent test cases.
	tests := []struct {
		useTabs, useCRLF bool
		ws, nl           string
	}{
		{false, false, " ", "\n"},
		{false, true, " ", "\r\n"},
		{true, false, "\t", "\n"},
		{true, true, "\t", "\r\n"},
	}

	for _, test := range tests {
		if test.useTabs {
			settings := NewIndentSettings()
			settings.UseTabs = true
			settings.UseCRLF = test.useCRLF
			doc.IndentWithSettings(settings)
			s, err := doc.WriteToString()
			if err != nil {
				t.Error("etree: failed to serialize document")
			}
			tab := test.ws
			expected := "<root>" + test.nl + tab + "<child1>" + test.nl +
				tab + tab + "<child2/>" + test.nl + tab +
				"</child1>" + test.nl + "</root>" + test.nl
			checkStrEq(t, s, expected)
		} else {
			for i := 0; i < 256; i++ {
				settings := NewIndentSettings()
				settings.Spaces = i
				settings.UseTabs = false
				settings.UseCRLF = test.useCRLF
				doc.IndentWithSettings(settings)
				s, err := doc.WriteToString()
				if err != nil {
					t.Error("etree: failed to serialize document")
				}
				tab := strings.Repeat(test.ws, i)
				expected := "<root>" + test.nl + tab + "<child1>" + test.nl +
					tab + tab + "<child2/>" + test.nl + tab +
					"</child1>" + test.nl + "</root>" + test.nl
				checkStrEq(t, s, expected)
			}
		}
	}
}

func TestIndentPreserveWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<test></test>", "<test/>"},
		{"<test>  </test>", "<test>  </test>"},
		{"<test>\t</test>", "<test>\t</test>"},
		{"<test>\t\n \t</test>", "<test>\t\n \t</test>"},
		{"<test><![CDATA[ ]]></test>", "<test> </test>"},
		{"<test> <![CDATA[ ]]> </test>", "<test/>"},
		{"<outer> <inner> </inner> </outer>", "<outer>\n  <inner> </inner>\n</outer>"},
	}

	for _, test := range tests {
		doc := NewDocument()
		err := doc.ReadFromString(test.input)
		if err != nil {
			t.Error("etree: failed to read string")
		}

		s := NewIndentSettings()
		s.Spaces = 2
		s.PreserveLeafWhitespace = true
		s.SuppressTrailingWhitespace = true
		doc.IndentWithSettings(s)

		output, err := doc.WriteToString()
		if err != nil {
			t.Error("etree: failed to read string")
		}
		checkStrEq(t, output, test.expected)
	}
}

func TestPreserveCData(t *testing.T) {
	tests := []struct {
		input                   string
		expectedWithPreserve    string
		expectedWithoutPreserve string
	}{
		{
			"<test><![CDATA[x]]></test>",
			"<test><![CDATA[x]]></test>",
			"<test>x</test>",
		},
		{
			"<tag><![CDATA[x <b>foo</b>]]></tag>",
			"<tag><![CDATA[x <b>foo</b>]]></tag>",
			"<tag>x &lt;b&gt;foo&lt;/b&gt;</tag>",
		},
		{
			"<name><![CDATA[My]]> <b>name</b> <![CDATA[is]]></name>",
			"<name><![CDATA[My]]> <b>name</b> <![CDATA[is]]></name>",
			"<name>My <b>name</b> is</name>",
		},
	}

	for _, test := range tests {
		doc := newDocumentFromString2(t, test.input, ReadSettings{PreserveCData: true})
		output, _ := doc.WriteToString()
		checkStrEq(t, output, test.expectedWithPreserve)
	}

	for _, test := range tests {
		doc := newDocumentFromString2(t, test.input, ReadSettings{PreserveCData: false})
		output, _ := doc.WriteToString()
		checkStrEq(t, output, test.expectedWithoutPreserve)
	}
}

func TestTokenIndexing(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="style.xsl"?>
<store xmlns:t="urn:books-com:titles">
	<!Directive>
	<!--This is a comment-->
	<book lang="en">
		<t:title>Great Expectations</t:title>
		<author>Charles Dickens</author>
		<review/>
	</book>
</store>`

	doc := newDocumentFromString(t, s)
	review := doc.FindElement("/store/book/review")
	review.SetText("Excellent")

	checkIndexes(t, &doc.Element)

	doc.Indent(4)
	checkIndexes(t, &doc.Element)

	doc.Indent(NoIndent)
	checkIndexes(t, &doc.Element)

	e := NewElement("foo")
	store := doc.SelectElement("store")
	store.InsertChildAt(0, e)
	checkIndexes(t, &doc.Element)

	store.RemoveChildAt(0)
	checkIndexes(t, &doc.Element)
}

func TestSetText(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateElement("root")

	checkDocEq(t, doc, `<root/>`)
	checkStrEq(t, root.Text(), "")
	checkIntEq(t, len(root.Child), 0)

	root.SetText("foo")
	checkDocEq(t, doc, `<root>foo</root>`)
	checkStrEq(t, root.Text(), "foo")
	checkIntEq(t, len(root.Child), 1)

	root.SetText("bar")
	checkDocEq(t, doc, `<root>bar</root>`)
	checkStrEq(t, root.Text(), "bar")
	checkIntEq(t, len(root.Child), 1)

	root.CreateCData("cdata")
	checkDocEq(t, doc, `<root>bar<![CDATA[cdata]]></root>`)
	checkStrEq(t, root.Text(), "barcdata")
	checkIntEq(t, len(root.Child), 2)

	root.SetText("qux")
	checkDocEq(t, doc, `<root>qux</root>`)
	checkStrEq(t, root.Text(), "qux")
	checkIntEq(t, len(root.Child), 1)

	root.CreateCData("cdata")
	checkDocEq(t, doc, `<root>qux<![CDATA[cdata]]></root>`)
	checkStrEq(t, root.Text(), "quxcdata")
	checkIntEq(t, len(root.Child), 2)

	root.SetCData("baz")
	checkDocEq(t, doc, `<root><![CDATA[baz]]></root>`)
	checkStrEq(t, root.Text(), "baz")
	checkIntEq(t, len(root.Child), 1)

	root.CreateText("corge")
	root.CreateCData("grault")
	root.CreateText("waldo")
	root.CreateCData("fred")
	root.CreateElement("child")
	checkDocEq(t, doc, `<root><![CDATA[baz]]>corge<![CDATA[grault]]>waldo<![CDATA[fred]]><child/></root>`)
	checkStrEq(t, root.Text(), "bazcorgegraultwaldofred")
	checkIntEq(t, len(root.Child), 6)

	root.SetText("plugh")
	checkDocEq(t, doc, `<root>plugh<child/></root>`)
	checkStrEq(t, root.Text(), "plugh")
	checkIntEq(t, len(root.Child), 2)

	root.SetText("")
	checkDocEq(t, doc, `<root><child/></root>`)
	checkStrEq(t, root.Text(), "")
	checkIntEq(t, len(root.Child), 1)

	root.SetText("")
	checkDocEq(t, doc, `<root><child/></root>`)
	checkStrEq(t, root.Text(), "")
	checkIntEq(t, len(root.Child), 1)

	root.RemoveChildAt(0)
	root.CreateText("corge")
	root.CreateCData("grault")
	root.CreateText("waldo")
	root.CreateCData("fred")
	root.CreateElement("child")
	checkDocEq(t, doc, `<root>corge<![CDATA[grault]]>waldo<![CDATA[fred]]><child/></root>`)
	checkStrEq(t, root.Text(), "corgegraultwaldofred")
	checkIntEq(t, len(root.Child), 5)

	root.SetText("")
	checkDocEq(t, doc, `<root><child/></root>`)
	checkStrEq(t, root.Text(), "")
	checkIntEq(t, len(root.Child), 1)
}

func TestSetTail(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateElement("root")
	child := root.CreateElement("child")
	root.CreateText("\n\t")
	child.SetText("foo")
	checkDocEq(t, doc, "<root><child>foo</child>\n\t</root>")
	checkStrEq(t, child.Tail(), "\n\t")
	checkIntEq(t, len(root.Child), 2)
	checkIntEq(t, len(child.Child), 1)

	root.CreateCData("    ")
	checkDocEq(t, doc, "<root><child>foo</child>\n\t<![CDATA[    ]]></root>")
	checkStrEq(t, child.Tail(), "\n\t    ")
	checkIntEq(t, len(root.Child), 3)
	checkIntEq(t, len(child.Child), 1)

	child.SetTail("")
	checkDocEq(t, doc, "<root><child>foo</child></root>")
	checkStrEq(t, child.Tail(), "")
	checkIntEq(t, len(root.Child), 1)
	checkIntEq(t, len(child.Child), 1)

	child.SetTail("\t\t\t")
	checkDocEq(t, doc, "<root><child>foo</child>\t\t\t</root>")
	checkStrEq(t, child.Tail(), "\t\t\t")
	checkIntEq(t, len(root.Child), 2)
	checkIntEq(t, len(child.Child), 1)

	child.SetTail("\t\n\n\t")
	checkDocEq(t, doc, "<root><child>foo</child>\t\n\n\t</root>")
	checkStrEq(t, child.Tail(), "\t\n\n\t")
	checkIntEq(t, len(root.Child), 2)
	checkIntEq(t, len(child.Child), 1)

	child.SetTail("")
	checkDocEq(t, doc, "<root><child>foo</child></root>")
	checkStrEq(t, child.Tail(), "")
	checkIntEq(t, len(root.Child), 1)
	checkIntEq(t, len(child.Child), 1)
}

func TestAttrParent(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateElement("root")
	attr1 := root.CreateAttr("bar", "1")
	attr2 := root.CreateAttr("qux", "2")

	checkIntEq(t, len(root.Attr), 2)
	checkElementEq(t, attr1.Element(), root)
	checkElementEq(t, attr2.Element(), root)

	attr1 = root.RemoveAttr("bar")
	attr2 = root.RemoveAttr("qux")
	checkElementEq(t, attr1.Element(), nil)
	checkElementEq(t, attr2.Element(), nil)

	s := `<root a="1" b="2" c="3" d="4"/>`
	err := doc.ReadFromString(s)
	if err != nil {
		t.Error("etree: failed to parse document")
	}

	root = doc.SelectElement("root")
	for i := range root.Attr {
		checkElementEq(t, root.Attr[i].Element(), root)
	}
}

func TestDefaultNamespaceURI(t *testing.T) {
	s := `
<root xmlns="https://root.example.com" xmlns:attrib="https://attrib.example.com" attrib:a="foo" b="bar">
	<child1 xmlns="https://child.example.com" attrib:a="foo">
		<grandchild1 xmlns="https://grandchild.example.com" a="foo">
		</grandchild1>
		<grandchild2 a="foo">
			<greatgrandchild1 attrib:a="foo"/>
		</grandchild2>
	</child1>
	<child2 a="foo"/>
</root>`

	doc := newDocumentFromString(t, s)
	root := doc.SelectElement("root")
	child1 := root.SelectElement("child1")
	child2 := root.SelectElement("child2")
	grandchild1 := child1.SelectElement("grandchild1")
	grandchild2 := child1.SelectElement("grandchild2")
	greatgrandchild1 := grandchild2.SelectElement("greatgrandchild1")

	checkStrEq(t, doc.NamespaceURI(), "")
	checkStrEq(t, root.NamespaceURI(), "https://root.example.com")
	checkStrEq(t, child1.NamespaceURI(), "https://child.example.com")
	checkStrEq(t, child2.NamespaceURI(), "https://root.example.com")
	checkStrEq(t, grandchild1.NamespaceURI(), "https://grandchild.example.com")
	checkStrEq(t, grandchild2.NamespaceURI(), "https://child.example.com")
	checkStrEq(t, greatgrandchild1.NamespaceURI(), "https://child.example.com")

	checkStrEq(t, root.Attr[0].NamespaceURI(), "")
	checkStrEq(t, root.Attr[1].NamespaceURI(), "")
	checkStrEq(t, root.Attr[2].NamespaceURI(), "https://attrib.example.com")
	checkStrEq(t, root.Attr[3].NamespaceURI(), "")
	checkStrEq(t, child1.Attr[0].NamespaceURI(), "")
	checkStrEq(t, child1.Attr[1].NamespaceURI(), "https://attrib.example.com")
	checkStrEq(t, child2.Attr[0].NamespaceURI(), "")
	checkStrEq(t, grandchild1.Attr[0].NamespaceURI(), "")
	checkStrEq(t, grandchild1.Attr[1].NamespaceURI(), "")
	checkStrEq(t, grandchild2.Attr[0].NamespaceURI(), "")
	checkStrEq(t, greatgrandchild1.Attr[0].NamespaceURI(), "https://attrib.example.com")

	f := doc.FindElements("//*[namespace-uri()='https://root.example.com']")
	if len(f) != 2 || f[0] != root || f[1] != child2 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='https://child.example.com']")
	if len(f) != 3 || f[0] != child1 || f[1] != grandchild2 || f[2] != greatgrandchild1 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='https://grandchild.example.com']")
	if len(f) != 1 || f[0] != grandchild1 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='']")
	if len(f) != 0 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='foo']")
	if len(f) != 0 {
		t.Error("etree: failed namespace-uri test")
	}
}

func TestLocalNamespaceURI(t *testing.T) {
	s := `
<a:root xmlns:a="https://root.example.com">
	<b:child1 xmlns:b="https://child.example.com">
		<c:grandchild1 xmlns:c="https://grandchild.example.com"/>
		<b:grandchild2>
			<a:greatgrandchild1/>
		</b:grandchild2>
		<a:grandchild3/>
		<grandchild4/>
	</b:child1>
	<a:child2>
	</a:child2>
	<child3>
	</child3>
</a:root>`

	doc := newDocumentFromString(t, s)
	root := doc.SelectElement("root")
	child1 := root.SelectElement("child1")
	child2 := root.SelectElement("child2")
	child3 := root.SelectElement("child3")
	grandchild1 := child1.SelectElement("grandchild1")
	grandchild2 := child1.SelectElement("grandchild2")
	grandchild3 := child1.SelectElement("grandchild3")
	grandchild4 := child1.SelectElement("grandchild4")
	greatgrandchild1 := grandchild2.SelectElement("greatgrandchild1")

	checkStrEq(t, doc.NamespaceURI(), "")
	checkStrEq(t, root.NamespaceURI(), "https://root.example.com")
	checkStrEq(t, child1.NamespaceURI(), "https://child.example.com")
	checkStrEq(t, child2.NamespaceURI(), "https://root.example.com")
	checkStrEq(t, child3.NamespaceURI(), "")
	checkStrEq(t, grandchild1.NamespaceURI(), "https://grandchild.example.com")
	checkStrEq(t, grandchild2.NamespaceURI(), "https://child.example.com")
	checkStrEq(t, grandchild3.NamespaceURI(), "https://root.example.com")
	checkStrEq(t, grandchild4.NamespaceURI(), "")
	checkStrEq(t, greatgrandchild1.NamespaceURI(), "https://root.example.com")

	f := doc.FindElements("//*[namespace-uri()='https://root.example.com']")
	if len(f) != 4 || f[0] != root || f[1] != child2 || f[2] != grandchild3 || f[3] != greatgrandchild1 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='https://child.example.com']")
	if len(f) != 2 || f[0] != child1 || f[1] != grandchild2 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='https://grandchild.example.com']")
	if len(f) != 1 || f[0] != grandchild1 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='']")
	if len(f) != 2 || f[0] != child3 || f[1] != grandchild4 {
		t.Error("etree: failed namespace-uri test")
	}

	f = doc.FindElements("//*[namespace-uri()='foo']")
	if len(f) != 0 {
		t.Error("etree: failed namespace-uri test")
	}
}

func TestWhitespace(t *testing.T) {
	s := "<root>\n\t<child>\n\t\t<grandchild> x</grandchild>\n    </child>\n</root>"

	doc := newDocumentFromString(t, s)
	root := doc.Root()
	checkIntEq(t, len(root.Child), 3)

	cd := root.Child[0].(*CharData)
	checkBoolEq(t, cd.IsWhitespace(), true)
	checkStrBinaryEq(t, cd.Data, "\n\t")

	cd = root.Child[2].(*CharData)
	checkBoolEq(t, cd.IsWhitespace(), true)
	checkStrBinaryEq(t, cd.Data, "\n")

	child := root.SelectElement("child")
	checkIntEq(t, len(child.Child), 3)

	cd = child.Child[0].(*CharData)
	checkBoolEq(t, cd.IsWhitespace(), true)
	checkStrBinaryEq(t, cd.Data, "\n\t\t")

	cd = child.Child[2].(*CharData)
	checkBoolEq(t, cd.IsWhitespace(), true)
	checkStrBinaryEq(t, cd.Data, "\n    ")

	grandchild := child.SelectElement("grandchild")
	checkIntEq(t, len(grandchild.Child), 1)

	cd = grandchild.Child[0].(*CharData)
	checkBoolEq(t, cd.IsWhitespace(), false)

	cd.SetData(" ")
	checkBoolEq(t, cd.IsWhitespace(), true)

	cd.SetData("        x")
	checkBoolEq(t, cd.IsWhitespace(), false)

	cd.SetData("\t\n\r    ")
	checkBoolEq(t, cd.IsWhitespace(), true)

	cd.SetData("\uFFFD")
	checkBoolEq(t, cd.IsWhitespace(), false)

	cd.SetData("")
	checkBoolEq(t, cd.IsWhitespace(), true)
}

func TestTokenWriteTo(t *testing.T) {
	s := `<store>
	<!-- comment -->
	<book>
		<title>Great Expectations</title>
	</book>
</store>`
	doc := newDocumentFromString(t, s)

	writeSettings := WriteSettings{}
	indentSettings := IndentSettings{UseTabs: true}

	tests := []struct {
		path     string
		expected string
	}{
		{"//store", "<store>\n\t<!-- comment -->\n\t<book>\n\t\t<title>Great Expectations</title>\n\t</book>\n</store>"},
		{"//store/book", "<book>\n\t<title>Great Expectations</title>\n</book>"},
		{"//store/book/title", "<title>Great Expectations</title>"},
	}
	for _, test := range tests {
		var buffer bytes.Buffer

		c := doc.FindElement(test.path)
		c.IndentWithSettings(&indentSettings)
		c.WriteTo(&buffer, &writeSettings)
		checkStrEq(t, buffer.String(), test.expected)
	}
}

func TestReindexChildren(t *testing.T) {
	s := `<root>
	<child1/>
	<child2/>
	<child3/>
	<child4/>
	<child5/>
</root>`
	doc := newDocumentFromString(t, s)
	doc.Unindent()

	root := doc.Root()
	if root == nil || root.Tag != "root" || len(root.Child) != 5 {
		t.Error("etree: expected root element not found")
	}

	for i := 0; i < len(root.Child); i++ {
		if root.Child[i].Index() != i {
			t.Error("etree: incorrect child index found in root element child")
		}
	}

	rand.Shuffle(len(root.Child), func(i, j int) {
		root.Child[i], root.Child[j] = root.Child[j], root.Child[i]
	})

	root.ReindexChildren()

	for i := 0; i < len(root.Child); i++ {
		if root.Child[i].Index() != i {
			t.Error("etree: incorrect child index found in root element child")
		}
	}
}

func TestPreserveDuplicateAttrs(t *testing.T) {
	s := `<element x="value1" y="value2" x="value3" x="value4" y="value5"/>`

	checkAttrCount := func(e *Element, n int) {
		if len(e.Attr) != n {
			t.Errorf("etree: expected %d attributes, got %d", n, len(e.Attr))
		}
	}
	checkAttr := func(e *Element, i int, key, value string) {
		if i >= len(e.Attr) {
			t.Errorf("etree: attr[%d] out of bounds", i)
			return
		}
		if e.Attr[i].Key != key {
			t.Errorf("etree: attr[%d] expected key %s, got %s", i, key, e.Attr[i].Key)
		}
		if e.Attr[i].Value != value {
			t.Errorf("etree: attr[%d] expected value %s, got %s", i, value, e.Attr[i].Value)
		}
	}

	t.Run("enabled", func(t *testing.T) {
		doc := newDocumentFromString2(t, s, ReadSettings{PreserveDuplicateAttrs: true})
		e := doc.FindElement("element")
		checkAttrCount(e, 5)
		checkAttr(e, 0, "x", "value1")
		checkAttr(e, 1, "y", "value2")
		checkAttr(e, 2, "x", "value3")
		checkAttr(e, 3, "x", "value4")
		checkAttr(e, 4, "y", "value5")
	})

	t.Run("disabled", func(t *testing.T) {
		doc := newDocumentFromString2(t, s, ReadSettings{})
		e := doc.FindElement("element")
		checkAttrCount(e, 2)
		checkAttr(e, 0, "x", "value4")
		checkAttr(e, 1, "y", "value5")
	})
}

func TestNotNil(t *testing.T) {
	s := `<enabled>true</enabled>`

	doc := newDocumentFromString(t, s)
	doc.SelectElement("enabled").NotNil().SetText("false")
	doc.SelectElement("visible").NotNil().SetText("true")

	want := `<enabled>false</enabled>`
	got, err := doc.WriteToString()
	if err != nil {
		t.Fatal("etree: failed to write document to string")
	}
	if got != want {
		t.Error("etree: unexpected NotNil result")
		t.Error("wanted:\n" + want)
		t.Error("got:\n" + got)
	}
}

func TestValidateInput(t *testing.T) {
	tests := []struct {
		s   string
		err string
	}{
		{`<root>x</root>`, ""},
		{`<root/>`, ""},
		{`<root>x`, `XML syntax error on line 1: unexpected EOF`},
		{`</root><root>`, `XML syntax error on line 1: unexpected end element </root>`},
		{`<>`, `XML syntax error on line 1: expected element name after <`},
		{`<root>x</root>trailing`, "etree: invalid XML format"},
		{`<root>x</root><`, "etree: invalid XML format"},
		{`<root><child>x</child></root1>`, `XML syntax error on line 1: element <root> closed by </root1>`},
	}

	type readFunc func(doc *Document, s string) error
	runTests := func(t *testing.T, read readFunc) {
		for i, test := range tests {
			doc := NewDocument()
			doc.ReadSettings.ValidateInput = true
			err := read(doc, test.s)
			if err == nil {
				if test.err != "" {
					t.Errorf("etree: test #%d:\nExpected error:\n  %s\nReceived error:\n  nil", i, test.err)
				}
				root := doc.Root()
				if root == nil || root.Tag != "root" {
					t.Errorf("etree: test #%d: failed to read document after input validation", i)
				}
			} else {
				te := err.Error()
				if te != test.err {
					t.Errorf("etree: test #%d:\nExpected error;\n  %s\nReceived error:\n  %s", i, test.err, te)
				}
			}
		}
	}

	readFromString := func(doc *Document, s string) error {
		return doc.ReadFromString(s)
	}
	t.Run("ReadFromString", func(t *testing.T) { runTests(t, readFromString) })

	readFromBytes := func(doc *Document, s string) error {
		return doc.ReadFromBytes([]byte(s))
	}
	t.Run("ReadFromBytes", func(t *testing.T) { runTests(t, readFromBytes) })

	readFromFile := func(doc *Document, s string) error {
		pathtmp := path.Join(t.TempDir(), "etree-test")
		err := os.WriteFile(pathtmp, []byte(s), fs.ModePerm)
		if err != nil {
			return errors.New("unable to write tmp file for input validation")
		}
		return doc.ReadFromFile(pathtmp)
	}
	t.Run("ReadFromFile", func(t *testing.T) { runTests(t, readFromFile) })
}

func TestSiblingElement(t *testing.T) {
	doc := newDocumentFromString(t, `<root><a/><b>  <b1/> </b> <!--test--> <c/></root>`)

	root := doc.SelectElement("root")
	a := root.SelectElement("a")
	b := root.SelectElement("b")
	c := root.SelectElement("c")
	b1 := b.SelectElement("b1")

	tests := []struct {
		e    *Element
		next *Element
		prev *Element
	}{
		{root, nil, nil},
		{a, b, nil},
		{b, c, a},
		{c, nil, b},
		{b1, nil, nil},
	}

	toString := func(e *Element) string {
		if e == nil {
			return "nil"
		}
		return e.Tag
	}

	for i, test := range tests {
		next := test.e.NextSibling()
		if next != test.next {
			t.Errorf("etree: test #%d unexpected NextSibling result.\n  Expected: %s\n  Received: %s\n",
				i, toString(next), toString(test.next))
		}

		prev := test.e.PrevSibling()
		if prev != test.prev {
			t.Errorf("etree: test #%d unexpected PrevSibling result.\n  Expected: %s\n  Received: %s\n",
				i, toString(prev), toString(test.prev))
		}
	}
}

func TestContinuations(t *testing.T) {
	doc := NewDocument()
	root := doc.CreateChild("root", func(e *Element) {
		e.CreateChild("child1", func(e *Element) {
			e.CreateComment("Grandchildren of child #1")
			e.CreateChild("grandchild1", func(e *Element) {
				e.CreateAttr("attr1", "1")
				e.CreateAttr("attr2", "2")
			})
			e.CreateChild("grandchild2", func(e *Element) {
				e.CreateAttr("attr1", "3")
				e.CreateAttr("attr2", "4")
			})
		})
		e.CreateChild("child2", func(e *Element) {
			e.CreateComment("Grandchildren of child #2")
			e.CreateChild("grandchild1", func(e *Element) {
				e.CreateAttr("attr1", "5")
				e.CreateAttr("attr2", "6")
			})
			e.CreateChild("grandchild2", func(e *Element) {
				e.CreateAttr("attr1", "7")
				e.CreateAttr("attr2", "8")
			})
		})
	})
	checkStrEq(t, root.Tag, "root")

	// Serialize the document to a string
	doc.IndentTabs()
	s, err := doc.WriteToString()
	if err != nil {
		t.Error("etree: failed to serialize document")
	}

	// Make sure the serialized XML matches expectation.
	expected := `<root>
	<child1>
		<!--Grandchildren of child #1-->
		<grandchild1 attr1="1" attr2="2"/>
		<grandchild2 attr1="3" attr2="4"/>
	</child1>
	<child2>
		<!--Grandchildren of child #2-->
		<grandchild1 attr1="5" attr2="6"/>
		<grandchild2 attr1="7" attr2="8"/>
	</child2>
</root>
`

	checkStrEq(t, s, expected)
}
