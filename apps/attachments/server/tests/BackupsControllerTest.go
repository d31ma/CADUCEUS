import (
	"errors"
	"strings"
)

// @Test(BackupsTestSetup, {"partition":"tests"})
type BackupsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *BackupsControllerTest) backedUp(token string, version int64, content string) BackupUpload {
	upload, startErr := BackupsTestSetup{}.Controller().PutMine(token, BackupsTestSetup{}.Start(version, content, "phone"))
	BackupsTestSetup{}.Put(upload.UploadUrl, content)
	_, completeErr := BackupsTestSetup{}.Controller().PostComplete(token, BackupComplete{Version: upload.Version})
	Assert.Equals(nil, startErr)
	Assert.Equals(nil, completeErr)
	return upload
}

func (test *BackupsControllerTest) TestAPersonBacksUpAndDownloads() {
	domainId := BackupsTestSetup{}.Domain()
	ana := BackupsTestSetup{}.Tokens(domainId, []string{"ana"})["ana"]
	upload, startErr := BackupsTestSetup{}.Controller().PutMine(ana, BackupsTestSetup{}.Start(0, "cipher", "phone"))
	BackupsTestSetup{}.Put(upload.UploadUrl, "cipher")
	backup, completeErr := BackupsTestSetup{}.Controller().PostComplete(ana, BackupComplete{Version: 1})
	mine, mineErr := BackupsTestSetup{}.Controller().GetMine(ana)
	download, downloadErr := BackupsTestSetup{}.Controller().GetDownload(ana)
	Assert.Equals(nil, startErr)
	Assert.Equals(nil, completeErr)
	Assert.Equals(nil, mineErr)
	Assert.Equals(nil, downloadErr)
	Assert.Equals(int64(1), upload.Version)
	Assert.IsTrue(strings.Contains(upload.UploadUrl, "/"+domainId+"/backup-"), upload.UploadUrl)
	Assert.Equals(int64(1), backup.Version)
	Assert.Equals("phone", mine.UpdatedByDeviceId)
	Assert.Equals(1, len(mine.KeySlots))
	Assert.Equals(int64(600000), mine.KeySlots[0].Rounds)
	Assert.IsTrue(strings.Contains(download.Url, "/"+domainId+"/backup-"), download.Url)
}

func (test *BackupsControllerTest) TestANewVersionReplacesTheOldBytes() {
	ana := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})["ana"]
	first := test.backedUp(ana, 0, "first")
	second := test.backedUp(ana, 1, "second")
	mine, _ := BackupsTestSetup{}.Controller().GetMine(ana)
	Assert.Equals(int64(2), second.Version)
	Assert.Equals(int64(2), mine.Version)
	Assert.IsTrue(!BackupsTestSetup{}.Exists(first.UploadUrl), "the old version kept its bytes")
	Assert.IsTrue(BackupsTestSetup{}.Exists(second.UploadUrl), "the new version lost its bytes")
}

func (test *BackupsControllerTest) TestAStartWithAnOldVersionIsAConflict() {
	ana := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})["ana"]
	test.backedUp(ana, 0, "first")
	_, err := BackupsTestSetup{}.Controller().PutMine(ana, BackupsTestSetup{}.Start(0, "laptop", "laptop"))
	conflict := &AttachmentConflict{}
	Assert.IsTrue(errors.As(err, &conflict), "an old version was accepted")
	Assert.Equals("expectedVersion", conflict.Field)
	Assert.Equals(3519, conflict.Number())
}

func (test *BackupsControllerTest) TestCompleteWithTheWrongBytesIsAConflict() {
	ana := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})["ana"]
	upload, _ := BackupsTestSetup{}.Controller().PutMine(ana, BackupsTestSetup{}.Start(0, "cipher", "phone"))
	BackupsTestSetup{}.Put(upload.UploadUrl, "cipheR")
	_, err := BackupsTestSetup{}.Controller().PostComplete(ana, BackupComplete{Version: 1})
	conflict := &AttachmentConflict{}
	Assert.IsTrue(errors.As(err, &conflict), "wrong bytes accepted")
	Assert.Equals("sha256: The uploaded file does not match the SHA-256 you gave.", conflict.Error())
}

func (test *BackupsControllerTest) TestAnotherPersonGetsNotFound() {
	tokens := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana", "bo"})
	test.backedUp(tokens["ana"], 0, "cipher")
	_, mineErr := BackupsTestSetup{}.Controller().GetMine(tokens["bo"])
	_, downloadErr := BackupsTestSetup{}.Controller().GetDownload(tokens["bo"])
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(mineErr, &missing), "bo saw ana's backup")
	Assert.IsTrue(errors.As(downloadErr, &missing), "bo got a link to ana's backup")
	Assert.Equals(3514, missing.Number())
}

func (test *BackupsControllerTest) TestBadKeySlotsAreRefused() {
	ana := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})["ana"]
	request := BackupsTestSetup{}.Start(0, "cipher", "phone")
	request.KeySlots[0].Rounds = 10
	_, err := BackupsTestSetup{}.Controller().PutMine(ana, request)
	invalid := &InvalidUpload{}
	Assert.IsTrue(errors.As(err, &invalid), "weak slot accepted")
	Assert.Equals("keySlots", invalid.Field)
	Assert.Equals(3517, invalid.Number())
}

func (test *BackupsControllerTest) TestALargeBackupIsRefused() {
	ana := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})["ana"]
	_, err := BackupsTestSetup{}.Controller().PutMine(ana, BackupsTestSetup{}.Start(0, strings.Repeat("x", 51), "phone"))
	large := &AttachmentTooLarge{}
	Assert.IsTrue(errors.As(err, &large), "a large backup was accepted")
	Assert.Equals(int64(50), large.Limit)
}

func (test *BackupsControllerTest) TestDeleteRemovesTheBackup() {
	ana := BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})["ana"]
	upload := test.backedUp(ana, 0, "cipher")
	deleteErr := BackupsTestSetup{}.Controller().DeleteMine(ana)
	againErr := BackupsTestSetup{}.Controller().DeleteMine(ana)
	_, mineErr := BackupsTestSetup{}.Controller().GetMine(ana)
	missing := &AttachmentNotFound{}
	Assert.IsTrue(deleteErr == nil && againErr == nil, "delete failed")
	Assert.IsTrue(errors.As(mineErr, &missing), "the backup is still there")
	Assert.IsTrue(!BackupsTestSetup{}.Exists(upload.UploadUrl), "the bytes are still there")
}

func (test *BackupsControllerTest) TestARefusedTokenIsUnauthenticated() {
	BackupsTestSetup{}.Tokens(BackupsTestSetup{}.Domain(), []string{"ana"})
	_, err := BackupsTestSetup{}.Controller().GetMine("Bearer nonsense")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a bad token was accepted")
}
