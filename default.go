package rbxfetch

import (
	"github.com/anaminus/iofl"
)

func newDefaultMethods() map[string][]string {
	return map[string][]string{
		"Builds":             {"Builds"},
		"Latest":             {"Latest"},
		"APIDump":            {"APIDump"},
		"ReflectionMetadata": {"ReflectionMetadata"},
		"ClassImages":        {"ClassImages", "ExplorerIcons"},
		"Live":               {"Live64", "Live"},
	}
}

func newDefaultChainSet() *iofl.ChainSet {
	return iofl.NewChainSet(
		iofl.FilterDef{Name: "url", New: NewFilterURL},
		iofl.FilterDef{Name: "file", New: NewFilterFile},
		iofl.FilterDef{Name: "zip", New: NewFilterZip},
		iofl.FilterDef{Name: "iconscan", New: NewFilterIconScan},
	).MustSetConfig(
		iofl.Config{
			Chains: map[string]iofl.Chain{
				"Latest": {
					{Filter: "url", Params: iofl.Params{"URL": "https://setup.rbxcdn.com/versionQTStudio"}},
				},
				"Live": {
					{Filter: "url", Params: iofl.Params{"URL": "https://versioncompatibility.api.roblox.com/GetCurrentClientVersionUpload/?apiKey=76e5a40c-3ae1-4028-9f10-7c62520bd94f&binaryType=WindowsStudio"}},
				},
				"Live64": {
					{Filter: "url", Params: iofl.Params{"URL": "https://versioncompatibility.api.roblox.com/GetCurrentClientVersionUpload/?apiKey=76e5a40c-3ae1-4028-9f10-7c62520bd94f&binaryType=WindowsStudio64"}},
				},
				"Builds": {
					{Filter: "url", Params: iofl.Params{"URL": "https://setup.rbxcdn.com/DeployHistory.txt"}},
				},
				"APIDump": {
					{Filter: "url", Params: iofl.Params{"URL": "https://setup.rbxcdn.com/$GUID-API-Dump.json"}},
				},
				"ReflectionMetadata": {
					{Filter: "url", Params: iofl.Params{"URL": "https://setup.rbxcdn.com/$GUID-RobloxStudio.zip"}},
					{Filter: "zip", Params: iofl.Params{"File": "ReflectionMetadata.xml"}},
				},
				"ClassImages": {
					{Filter: "url", Params: iofl.Params{"URL": "https://setup.rbxcdn.com/$GUID-content-textures2.zip#ClassImages.PNG"}},
					{Filter: "zip", Params: iofl.Params{"File": "ClassImages.PNG"}},
				},
				"ExplorerIcons": {
					{Filter: "url", Params: iofl.Params{"URL": "https://setup.rbxcdn.com/$GUID-RobloxStudio.zip#RobloxStudioBeta.exe"}},
					{Filter: "zip", Params: iofl.Params{"File": "RobloxStudioBeta.exe"}},
					{Filter: "iconscan", Params: iofl.Params{"Size": 16}},
				},
			},
		},
	)
}
