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

	"android/soong/android"
	"android/soong/genrule"
)

func init() {
	android.RegisterModuleType("cc_genrule", GenRuleFactory)
}

type GenruleExtraProperties struct {
	Vendor_available         *bool
	Odm_available            *bool
	Product_available        *bool
	Ramdisk_available        *bool
	Vendor_ramdisk_available *bool
	Recovery_available       *bool
	Sdk_version              *string
}

// cc_genrule is a genrule that can depend on other cc_* objects.
// The cmd may be run multiple times, once for each of the different arch/etc
// variations.  The following environment variables will be set when the command
// execute:
//
//	CC_ARCH           the name of the architecture the command is being executed for
//
//	CC_MULTILIB       "lib32" if the architecture the command is being executed for is 32-bit,
//	                  "lib64" if it is 64-bit.
//
//	CC_NATIVE_BRIDGE  the name of the subdirectory that native bridge libraries are stored in if
//	                  the architecture has native bridge enabled, empty if it is disabled.
//
//	CC_OS             the name of the OS the command is being executed for.
func GenRuleFactory() android.Module {
	module := genrule.NewGenRule()

	extra := &GenruleExtraProperties{}
	module.Extra = extra
	module.ImageInterface = extra
	module.CmdModifier = genruleCmdModifier
	module.AddProperties(module.Extra)

	android.InitAndroidArchModule(module, android.HostAndDeviceSupported, android.MultilibBoth)

	android.InitApexModule(module)

	android.InitDefaultableModule(module)

	return module
}

func genruleCmdModifier(ctx android.ModuleContext, cmd string) string {
	target := ctx.Target()
	arch := target.Arch.ArchType
	osName := target.Os.Name
	return fmt.Sprintf("CC_ARCH=%s CC_NATIVE_BRIDGE=%s CC_MULTILIB=%s CC_OS=%s && %s",
		arch.Name, target.NativeBridgeRelativePath, arch.Multilib, osName, cmd)
}

var _ android.ImageInterface = (*GenruleExtraProperties)(nil)

func (g *GenruleExtraProperties) ImageMutatorBegin(ctx android.ImageInterfaceContext) {}

func (g *GenruleExtraProperties) VendorVariantNeeded(ctx android.ImageInterfaceContext) bool {
	return Bool(g.Vendor_available) || Bool(g.Odm_available) || ctx.SocSpecific() || ctx.DeviceSpecific()
}

func (g *GenruleExtraProperties) ProductVariantNeeded(ctx android.ImageInterfaceContext) bool {
	return Bool(g.Product_available) || ctx.ProductSpecific()
}

func (g *GenruleExtraProperties) CoreVariantNeeded(ctx android.ImageInterfaceContext) bool {
	return !(ctx.SocSpecific() || ctx.DeviceSpecific() || ctx.ProductSpecific())
}

func (g *GenruleExtraProperties) RamdiskVariantNeeded(ctx android.ImageInterfaceContext) bool {
	return Bool(g.Ramdisk_available)
}

func (g *GenruleExtraProperties) VendorRamdiskVariantNeeded(ctx android.ImageInterfaceContext) bool {
	return Bool(g.Vendor_ramdisk_available)
}

func (g *GenruleExtraProperties) DebugRamdiskVariantNeeded(ctx android.ImageInterfaceContext) bool {
	return false
}

func (g *GenruleExtraProperties) RecoveryVariantNeeded(ctx android.ImageInterfaceContext) bool {
	// If the build is using a snapshot, the recovery variant under AOSP directories
	// is not needed.
	return Bool(g.Recovery_available)
}

func (g *GenruleExtraProperties) ExtraImageVariations(ctx android.ImageInterfaceContext) []string {
	return nil
}

func (g *GenruleExtraProperties) SetImageVariation(ctx android.ImageInterfaceContext, variation string) {
}
