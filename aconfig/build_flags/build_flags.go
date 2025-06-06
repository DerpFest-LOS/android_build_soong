// Copyright 2024 Google Inc. All rights reserved.
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

package build_flags

import (
	"fmt"

	"android/soong/android"
)

const (
	outJsonFileName = "build_flags.json"
)

func init() {
	registerBuildFlagsModuleType(android.InitRegistrationContext)
}

func registerBuildFlagsModuleType(ctx android.RegistrationContext) {
	ctx.RegisterModuleType("build_flags_json", buildFlagsFactory)
}

type buildFlags struct {
	android.ModuleBase

	outputPath android.Path
}

func buildFlagsFactory() android.Module {
	module := &buildFlags{}
	android.InitAndroidArchModule(module, android.DeviceSupported, android.MultilibCommon)
	return module
}

func (m *buildFlags) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	// Read the build_flags_<partition>.json file generated by soong
	// 'release-config' command.
	srcPath := android.PathForOutput(ctx, "release-config", fmt.Sprintf("build_flags_%s.json", m.PartitionTag(ctx.DeviceConfig())))
	outputPath := android.PathForModuleOut(ctx, outJsonFileName)

	// The 'release-config' command is called for every build, and generates the
	// build_flags_<partition>.json file.
	// Update the output file only if the source file is changed.
	ctx.Build(pctx, android.BuildParams{
		Rule:   android.CpIfChanged,
		Input:  srcPath,
		Output: outputPath,
	})

	installPath := android.PathForModuleInstall(ctx, "etc")
	ctx.InstallFile(installPath, outJsonFileName, outputPath)
	m.outputPath = outputPath
}

func (m *buildFlags) AndroidMkEntries() []android.AndroidMkEntries {
	return []android.AndroidMkEntries{android.AndroidMkEntries{
		Class:      "ETC",
		OutputFile: android.OptionalPathForPath(m.outputPath),
	}}
}
