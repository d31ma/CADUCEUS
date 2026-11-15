// @Service
type BackupKeyStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *BackupKeyStorageService) ByPerson(personId string) (BackupKeyDocument, error) {
	key := BackupKeyDocument{}
	_, failure := storage.documents.FindOne(storage.names.BackupKeys(), map[string]any{storage.personField(): personId}, &key)
	return key, failure
}

func (storage *BackupKeyStorageService) Store(personId string, backupKey string) error {
	key, failure := storage.ByPerson(personId)
	key.PersonId = personId
	key.BackupKey = backupKey
	return Steps{}.Unless(failure, func() error { return storage.insertOrReplace(key) })
}

func (storage *BackupKeyStorageService) Delete(personId string) error {
	key, failure := storage.ByPerson(personId)
	return Steps{}.Unless(failure, func() error { return storage.deleteStored(key) })
}

func (storage *BackupKeyStorageService) insertOrReplace(key BackupKeyDocument) error {
	if key.Id == "" {
		_, failure := storage.documents.Insert(storage.names.BackupKeys(), key)
		return failure
	}
	saved := key
	saved.Id = ""
	return storage.documents.Replace(storage.names.BackupKeys(), key.Id, saved)
}

func (storage *BackupKeyStorageService) deleteStored(key BackupKeyDocument) error {
	if key.Id == "" {
		return nil
	}
	return storage.documents.Delete(storage.names.BackupKeys(), key.Id)
}

func (storage *BackupKeyStorageService) personField() string {
	return "personId"
}
