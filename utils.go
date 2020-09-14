// Copyright 2020 The xgen Authors. All rights reserved. Use of this source
// code is governed by a BSD-style license that can be found in the LICENSE
// file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// GetFileList get a list of file by given path.
func GetFileList(path string) (files []string, err error) {
	var fi os.FileInfo
	fi, err = os.Stat(path)
	if err != nil {
		return
	}
	if fi.IsDir() {
		err = filepath.Walk(path, func(fp string, info os.FileInfo, err error) error {
			files = append(files, fp)
			return nil
		})
		if err != nil {
			return
		}
	}
	files = append(files, path)
	return
}

// PrepareOutputDir provide a method to create the output directory by given
// path.
func PrepareOutputDir(path string) error {
	if path == "" {
		return nil
	}
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

// BuildInTypes defines the correspondence between Go, TypeScript, C, Java,
// Rust languages and data types in XSD.
// https://www.w3.org/TR/xmlschema-2/#datatype
var BuildInTypes = map[string][]string{
	"anyType":            {"string", "string", "char", "String", "char"},
	"ENTITIES":           {"[]string", "Array<string>", "char[]", "List<String>", "Vec<char>"},
	"ENTITY":             {"string", "string", "char", "String", "char"},
	"ID":                 {"string", "string", "char", "String", "char"},
	"IDREF":              {"string", "string", "char", "String", "char"},
	"IDREFS":             {"[]string", "Array<string>", "char[]", "List<String>", "Vec<char>"},
	"NCName":             {"string", "string", "char", "String", "char"},
	"NMTOKEN":            {"string", "string", "char", "String", "char"},
	"NMTOKENS":           {"[]string", "Array<string>", "char[]", "List<String>", "Vec<char>"},
	"NOTATION":           {"[]string", "Array<string>", "char[]", "List<String>", "Vec<char>"},
	"Name":               {"string", "string", "char", "String", "char"},
	"QName":              {"xml.Name", "any", "char", "String", "char"},
	"anyURI":             {"string", "string", "char", "QName", "char"},
	"base64Binary":       {"[]byte", "Array<any>", "char[]", "List<Byte>", "Vec<u8>"},
	"boolean":            {"bool", "boolean", "bool", "Boolean", "bool"},
	"byte":               {"byte", "any", "char[]", "Byte", "&[u8]"},
	"date":               {"time.Time", "string", "char", "Byte", "&[u8]"},
	"dateTime":           {"time.Time", "string", "char", "Byte", "&[u8]"},
	"decimal":            {"float64", "number", "float", "Float", "f64"},
	"double":             {"float64", "number", "float", "Float", "f64"},
	"duration":           {"string", "string", "char", "String", "char"},
	"float":              {"float", "number", "float", "Float", "usize"},
	"gDay":               {"time.Time", "string", "char", "String", "char"},
	"gMonth":             {"time.Time", "string", "char", "String", "char"},
	"gMonthDay":          {"time.Time", "string", "char", "String", "char"},
	"gYear":              {"time.Time", "string", "char", "String", "char"},
	"gYearMonth":         {"time.Time", "string", "char", "String", "char"},
	"hexBinary":          {"[]byte", "Array<any>", "char[]", "List<Byte>", "Vec<u8>"},
	"int":                {"int", "number", "int", "Integer", "isize"},
	"integer":            {"int", "number", "int", "Integer", "isize"},
	"language":           {"string", "string", "char", "String", "char"},
	"long":               {"int64", "number", "int", "Long", "i64"},
	"negativeInteger":    {"int", "number", "int", "Integer", "isize"},
	"nonNegativeInteger": {"int", "number", "int", "Integer", "isize"},
	"normalizedString":   {"string", "string", "char", "String", "char"},
	"nonPositiveInteger": {"int", "number", "int", "Integer", "isize"},
	"positiveInteger":    {"int", "number", "int", "Integer", "isize"},
	"short":              {"int16", "number", "int", "Integer", "i16"},
	"string":             {"string", "string", "char", "String", "char"},
	"time":               {"time.Time", "string", "char", "String", "char"},
	"token":              {"string", "string", "char", "String", "char"},
	"unsignedByte":       {"byte", "any", "char", "Byte", "&[u8]"},
	"unsignedInt":        {"uint32", "number", "unsigned int", "Integer", "u32"},
	"unsignedLong":       {"uint64", "number", "unsigned int", "Long", "u64"},
	"unsignedShort":      {"uint16", "number", "unsigned int", "Short", "u16"},
	"xml:lang":           {"string", "string", "char", "String", "char"},
	"xml:space":          {"string", "string", "char", "String", "char"},
	"xml:base":           {"string", "string", "char", "String", "char"},
	"xml:id":             {"string", "string", "char", "String", "char"},
}

func getBuildInTypeByLang(value, lang string) (buildType string, ok bool) {
	var supportLang = map[string]int{
		"Go":         0,
		"TypeScript": 1,
		"C":          2,
		"Java":       3,
		"Rust":       4,
	}
	var buildInTypes []string
	if buildInTypes, ok = BuildInTypes[value]; !ok {
		return
	}
	buildType = buildInTypes[supportLang[lang]]
	return
}
func getBasefromSimpleType(name string, XSDSchema []interface{}) string {
	for _, ele := range XSDSchema {
		switch v := ele.(type) {
		case *SimpleType:
			if !v.List && !v.Union && v.Name == name {
				return v.Base
			}
		case *Attribute:
			if v.Name == name {
				return v.Type
			}
		case *Element:
			if v.Name == name {
				return v.Type
			}
		}
	}
	return name
}

func getNSPrefix(str string) (ns string) {
	split := strings.Split(str, ":")
	if len(split) == 2 {
		ns = split[0]
		return
	}
	return
}

func trimNSPrefix(str string) (name string) {
	split := strings.Split(str, ":")
	if len(split) == 2 {
		name = split[1]
		return
	}
	name = str
	return
}

// MakeFirstUpperCase make the first letter of a string uppercase.
func MakeFirstUpperCase(s string) string {

	if len(s) < 2 {
		return strings.ToUpper(s)
	}

	bts := []byte(s)

	lc := bytes.ToUpper([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}

// callFuncByName calls the no error or only error return function with
// reflect by given receiver, name and parameters.
func callFuncByName(receiver interface{}, name string, params []reflect.Value) (err error) {
	function := reflect.ValueOf(receiver).MethodByName(name)
	if function.IsValid() {
		rt := function.Call(params)
		if len(rt) == 0 {
			return
		}
		if !rt[0].IsNil() {
			err = rt[0].Interface().(error)
			return
		}
	}
	return
}

// isValidUrl tests a string to determine if it is a well-structured url or
// not.
func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}

func fetchSchema(URL string) ([]byte, error) {
	var body []byte
	var client http.Client
	var err error
	resp, err := client.Get(URL)
	if err != nil {
		return body, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return body, err
		}
	}
	return body, err
}
