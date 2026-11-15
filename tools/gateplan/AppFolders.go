package main

type AppFolders []AppFolder

func (folders AppFolders) Named(name string) []string {
	for _, folder := range folders {
		if folder.Name == name {
			return []string{name}
		}
	}
	return nil
}

func (folders AppFolders) Listing(kitPackage string) []string {
	names := []string{}
	for _, folder := range folders {
		if folder.Lists(kitPackage) {
			names = append(names, folder.Name)
		}
	}
	return names
}
