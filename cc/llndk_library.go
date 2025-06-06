// Copyright 2017 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cc

import (
	"fmt"
	"strings"

	"android/soong/android"
	"android/soong/etc"
)

var (
	llndkLibrarySuffix = ".llndk"
)

// Holds properties to describe a stub shared library based on the provided version file.
type llndkLibraryProperties struct {
	// Relative path to the symbol map.
	// An example file can be seen here: TODO(danalbert): Make an example.
	Symbol_file *string `android:"path,arch_variant"`

	// Whether to export any headers as -isystem instead of -I. Mainly for use by
	// bionic/libc.
	Export_headers_as_system *bool

	// Whether the system library uses symbol versions.
	Unversioned *bool

	// list of llndk headers to re-export include directories from.
	Export_llndk_headers []string

	// list of directories relative to the Blueprints file that willbe added to the include path
	// (using -I) for any module that links against the LLNDK variant of this module, replacing
	// any that were listed outside the llndk clause.
	Override_export_include_dirs []string

	// whether this module can be directly depended upon by libs that are installed
	// to /vendor and /product.
	// When set to true, this module can only be depended on by VNDK libraries, not
	// vendor nor product libraries. This effectively hides this module from
	// non-system modules. Default value is false.
	Private *bool

	// if true, make this module available to provide headers to other modules that set
	// llndk.symbol_file.
	Llndk_headers *bool

	// moved_to_apex marks this module has having been distributed through an apex module.
	Moved_to_apex *bool
}

func makeLlndkVars(ctx android.MakeVarsContext) {
	// Make uses LLNDK_MOVED_TO_APEX_LIBRARIES to generate the linker config.
	movedToApexLlndkLibraries := make(map[string]bool)
	ctx.VisitAllModules(func(module android.Module) {
		if library := moduleLibraryInterface(module); library != nil && library.hasLLNDKStubs() {
			if library.isLLNDKMovedToApex() {
				name := library.implementationModuleName(module.(*Module).BaseModuleName())
				movedToApexLlndkLibraries[name] = true
			}
		}
	})

	ctx.Strict("LLNDK_MOVED_TO_APEX_LIBRARIES",
		strings.Join(android.SortedKeys(movedToApexLlndkLibraries), " "))
}

func init() {
	RegisterLlndkLibraryTxtType(android.InitRegistrationContext)
}

func RegisterLlndkLibraryTxtType(ctx android.RegistrationContext) {
	ctx.RegisterParallelSingletonModuleType("llndk_libraries_txt", llndkLibrariesTxtFactory)
}

type llndkLibrariesTxtModule struct {
	android.SingletonModuleBase

	outputFile  android.OutputPath
	moduleNames []string
	fileNames   []string
}

var _ etc.PrebuiltEtcModule = &llndkLibrariesTxtModule{}

// llndk_libraries_txt is a singleton module whose content is a list of LLNDK libraries
// generated by Soong but can be referenced by other modules.
// For example, apex_vndk can depend on these files as prebuilt.
// Make uses LLNDK_LIBRARIES to determine which libraries to install.
// HWASAN is only part of the LL-NDK in builds in which libc depends on HWASAN.
// Therefore, by removing the library here, we cause it to only be installed if libc
// depends on it.
func llndkLibrariesTxtFactory() android.SingletonModule {
	m := &llndkLibrariesTxtModule{}
	android.InitAndroidArchModule(m, android.DeviceSupported, android.MultilibCommon)
	return m
}

func (txt *llndkLibrariesTxtModule) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	filename := txt.Name()

	txt.outputFile = android.PathForModuleOut(ctx, filename).OutputPath

	installPath := android.PathForModuleInstall(ctx, "etc")
	ctx.InstallFile(installPath, filename, txt.outputFile)

	ctx.SetOutputFiles(android.Paths{txt.outputFile}, "")
}

func getVndkFileName(m *Module) (string, error) {
	if library, ok := m.linker.(*libraryDecorator); ok {
		return library.getLibNameHelper(m.BaseModuleName(), true, false) + ".so", nil
	}
	if prebuilt, ok := m.linker.(*prebuiltLibraryLinker); ok {
		return prebuilt.libraryDecorator.getLibNameHelper(m.BaseModuleName(), true, false) + ".so", nil
	}
	return "", fmt.Errorf("VNDK library should have libraryDecorator or prebuiltLibraryLinker as linker: %T", m.linker)
}

func (txt *llndkLibrariesTxtModule) GenerateSingletonBuildActions(ctx android.SingletonContext) {
	if txt.outputFile.String() == "" {
		// Skip if target file path is empty
		return
	}

	ctx.VisitAllModules(func(m android.Module) {
		if c, ok := m.(*Module); ok && c.VendorProperties.IsLLNDK && !c.Header() && !c.IsVndkPrebuiltLibrary() {
			filename, err := getVndkFileName(c)
			if err != nil {
				ctx.ModuleErrorf(m, "%s", err)
			}

			if !strings.HasPrefix(ctx.ModuleName(m), "libclang_rt.hwasan") {
				txt.moduleNames = append(txt.moduleNames, ctx.ModuleName(m))
			}
			txt.fileNames = append(txt.fileNames, filename)
		}
	})
	txt.moduleNames = android.SortedUniqueStrings(txt.moduleNames)
	txt.fileNames = android.SortedUniqueStrings(txt.fileNames)

	android.WriteFileRule(ctx, txt.outputFile, strings.Join(txt.fileNames, "\n"))
}

func (txt *llndkLibrariesTxtModule) AndroidMkEntries() []android.AndroidMkEntries {
	return []android.AndroidMkEntries{{
		Class:      "ETC",
		OutputFile: android.OptionalPathForPath(txt.outputFile),
		ExtraEntries: []android.AndroidMkExtraEntriesFunc{
			func(ctx android.AndroidMkExtraEntriesContext, entries *android.AndroidMkEntries) {
				entries.SetString("LOCAL_MODULE_STEM", txt.outputFile.Base())
			},
		},
	}}
}

func (txt *llndkLibrariesTxtModule) MakeVars(ctx android.MakeVarsContext) {
	ctx.Strict("LLNDK_LIBRARIES", strings.Join(txt.moduleNames, " "))
}

// PrebuiltEtcModule interface
func (txt *llndkLibrariesTxtModule) BaseDir() string {
	return "etc"
}

// PrebuiltEtcModule interface
func (txt *llndkLibrariesTxtModule) SubDir() string {
	return ""
}

func llndkMutator(mctx android.BottomUpMutatorContext) {
	m, ok := mctx.Module().(*Module)
	if !ok {
		return
	}

	if shouldSkipLlndkMutator(mctx, m) {
		return
	}

	lib, isLib := m.linker.(*libraryDecorator)
	prebuiltLib, isPrebuiltLib := m.linker.(*prebuiltLibraryLinker)

	if m.InVendorOrProduct() && isLib && lib.hasLLNDKStubs() {
		m.VendorProperties.IsLLNDK = true
	}
	if m.InVendorOrProduct() && isPrebuiltLib && prebuiltLib.hasLLNDKStubs() {
		m.VendorProperties.IsLLNDK = true
	}

	if vndkprebuilt, ok := m.linker.(*vndkPrebuiltLibraryDecorator); ok {
		if !Bool(vndkprebuilt.properties.Vndk.Enabled) {
			m.VendorProperties.IsLLNDK = true
		}
	}
}

// Check for modules that mustn't be LLNDK
func shouldSkipLlndkMutator(mctx android.BottomUpMutatorContext, m *Module) bool {
	if !m.Enabled(mctx) {
		return true
	}
	if !m.Device() {
		return true
	}
	if m.Target().NativeBridge == android.NativeBridgeEnabled {
		return true
	}
	return false
}
