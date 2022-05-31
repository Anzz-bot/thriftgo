// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package golang

import (
	"fmt"
	"strings"

	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/pkg/namespace"
)

type importManager struct {
	namespace.Namespace
	libNotUsed map[string]bool
}

func newImportManager() *importManager {
	im := &importManager{
		// The usage of these three libraries depends on specific code generation
		// and feature settings which requires tedisous type cheking before rendering.
		// So we register them into the namespace by addImports and mark them not-used,
		// use the UseStdLibrary to confirm if they are actually used.
		libNotUsed: map[string]bool{
			"strings": true,
			"bytes":   true,
			"reflect": true,
		},
	}
	return im
}

// ResolveImports returns a map of import path to alias built from the include list
// of the IDL. An alias may be an empty string to indicate no alias is need for the
// import path.
func (im *importManager) ResolveImports() (map[string]string, error) {
	imports := make(map[string]string)
	im.Iterate(func(alias, path string) bool {
		if im.libNotUsed[alias] {
			return true // skip
		}
		if alias == path || strings.HasSuffix(path, "/"+alias) {
			imports[path] = ""
		} else {
			imports[path] = alias
		}
		return true
	})
	return imports, nil
}

// UseStdLibrary claims to use a certain standard library.
// This function is designed to be called during template rendering to
// avoid tedious type checking for determine whether a library will be used.
func (im *importManager) UseStdLibrary(lib string) {
	delete(im.libNotUsed, lib)
}

func (im *importManager) init(cu *CodeUtils, ast *parser.Thrift) {
	im.Namespace = &idHijack{
		Namespace: namespace.NewNamespace(func(name string, cnt int) string {
			return fmt.Sprintf("%s%d", name, cnt-1) // zero-index
		}),
		replacement: cu.importReplace,
	}
	ns := im.Namespace

	if len(ast.Enums) > 0 {
		ns.Add("fmt", "fmt")
		if cu.Features().ScanValueForEnum {
			ns.Add("driver", "database/sql/driver")
			ns.Add("sql", "database/sql")
		}
	}

	if len(ast.GetStructLikes()) > 0 {
		ns.Add("fmt", "fmt")
		if !cu.features.DefinitionOnly {
			ns.Add("thrift", DefaultThriftLib)
		}
	}

	if len(ast.Services) > 0 {
		if !cu.features.DefinitionOnly {
			ns.Add("thrift", DefaultThriftLib)
		}
		for _, svc := range ast.Services {
			if svc.Extends == "" || len(svc.Functions) > 0 {
				ns.Add("context", "context")
			}
			if len(svc.Functions) > 0 {
				ns.Add("fmt", "fmt")
			}
		}
	}

	structCount := len(ast.GetStructLikes())
	ast.ForEachService(func(svc *parser.Service) bool {
		structCount += len(svc.Functions)
		return true
	})
	if structCount > 0 && cu.Features().KeepUnknownFields {
		ns.Add("unknown", DefaultUnknownLib)
	}

	if cu.Features().GenDeepEqual {
		ns.Add("strings", "strings")
		ns.Add("bytes", "bytes")
	} else if cu.Features().ValidateSet {
		ns.Add("reflect", "reflect")
	}
}

type idHijack struct {
	namespace.Namespace
	replacement map[string]string
}

func (h *idHijack) get(id string) string {
	if v, ok := h.replacement[id]; ok {
		return v
	}
	return id
}

func (h *idHijack) Add(name, id string) (result string) {
	return h.Namespace.Add(name, h.get(id))
}

func (h *idHijack) Get(id string) (name string) {
	return h.Namespace.Get(h.get(id))
}

func (h *idHijack) Reserve(name, id string) (ok bool) {
	return h.Namespace.Reserve(name, h.get(id))
}

func (h *idHijack) MustReserve(name, id string) {
	h.Namespace.MustReserve(name, h.get(id))
}
