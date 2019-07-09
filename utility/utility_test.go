package utility_test

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wal-g/wal-g/internal"
	"github.com/wal-g/wal-g/internal/ioextensions"
	"github.com/wal-g/wal-g/testtools"
	"github.com/wal-g/wal-g/utility"
)

const (
	CreateFileWithPath = "../test/testdata/createFileWith"
)

var times = []struct {
	input internal.BackupTime
}{
	{internal.BackupTime{
		BackupName:  "second",
		Time:        time.Date(2017, 2, 2, 30, 48, 39, 651387233, time.UTC),
		WalFileName: "",
	}},
	{internal.BackupTime{
		BackupName:  "fourth",
		Time:        time.Date(2009, 2, 27, 20, 8, 33, 651387235, time.UTC),
		WalFileName: "",
	}},
	{internal.BackupTime{
		BackupName:  "fifth",
		Time:        time.Date(2008, 11, 20, 16, 34, 58, 651387232, time.UTC),
		WalFileName: "",
	}},
	{internal.BackupTime{
		BackupName:  "first",
		Time:        time.Date(2020, 11, 31, 20, 3, 58, 651387237, time.UTC),
		WalFileName: "",
	}},
	{internal.BackupTime{
		BackupName:  "third",
		Time:        time.Date(2009, 3, 13, 4, 2, 42, 651387234, time.UTC),
		WalFileName: "",
	}},
}

func TestSortLatestTime(t *testing.T) {
	correct := [5]string{"first", "second", "third", "fourth", "fifth"}
	sortTimes := make([]internal.BackupTime, 5)

	for i, val := range times {
		sortTimes[i] = val.input
	}

	sort.Slice(sortTimes, func(i, j int) bool {
		return sortTimes[i].Time.After(sortTimes[j].Time)
	})

	for i, val := range sortTimes {
		assert.Equal(t, correct[i], val.BackupName)
	}
}

// Tests that backup name is successfully extracted from
// return values of pg_stop_backup(false)
func TestCheckType(t *testing.T) {
	var fileNames = []struct {
		input    string
		expected string
	}{
		{"mock.lzo", "lzo"},
		{"mock.tar.lzo", "lzo"},
		{"mock.gzip", "gzip"},
		{"mockgzip", ""},
	}
	for _, f := range fileNames {
		actual := utility.GetFileExtension(f.input)
		assert.Equal(t, f.expected, actual)
	}
}

func TestCreateFileWith(t *testing.T) {
	content := "content"
	err := ioextensions.CreateFileWith(CreateFileWithPath, strings.NewReader(content))
	assert.NoError(t, err)
	actualContent, err := ioutil.ReadFile(CreateFileWithPath)
	assert.NoError(t, err)
	assert.Equal(t, []byte(content), actualContent)
	os.Remove(CreateFileWithPath)
}

func TestCreateFileWith_ExistenceError(t *testing.T) {
	file, err := os.Create(CreateFileWithPath)
	assert.NoError(t, err)
	file.Close()
	err = ioextensions.CreateFileWith(CreateFileWithPath, strings.NewReader("error"))
	assert.Equal(t, os.IsExist(err), true)
	os.Remove(CreateFileWithPath)
}

func TestStripBackupName(t *testing.T) {
	var testCases = []struct {
		input    string
		expected string
	}{
		{"file_backup", "file"},
		{"backup", "backup"},
		{"/other_backup", "other"},
		{"path/to/tables_backup", "tables"},
		{"anotherPath/to/document_backup_backup", "document"},
		{"anotherPath/to/fileBackup", "fileBackup"},
	}

	for _, testCase := range testCases {
		actual := utility.StripBackupName(testCase.input)
		assert.Equal(t, testCase.expected, actual)
	}
}

func TestStripPrefixName(t *testing.T) {
	var testCases = []struct {
		input    string
		expected string
	}{
		{"//path/path1//", "path1"},
		{"//path//path1/", "path1"},
		{"path/path1", "path1"},
		{"path/path1/path2", "path2"},
		{"path/path1//	/path2", "path2"},
		{"", ""},
		{"/", ""},
	}

	for _, testCase := range testCases {
		actual := utility.StripPrefixName(testCase.input)
		assert.Equal(t, testCase.expected, actual)
	}
}

func TestCeilTimeUpToMicroseconds_Works_When_Nanoseconds_Greater_Than_Zero(t *testing.T) {
	timeToCeil := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	expectedTime := time.Date(2000, 1, 1, 1, 1, 1, 1000, time.UTC)
	assert.Equal(t, expectedTime, utility.CeilTimeUpToMicroseconds(timeToCeil))
}

func TestCeilTimeUpToMicroseconds_Works_When_Nanoseconds_Equal_Zero(t *testing.T) {
	timeToCeil := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
	assert.Equal(t, timeToCeil, utility.CeilTimeUpToMicroseconds(timeToCeil))
}

func TestFastCopy_NormalCases(t *testing.T) {
	var testDataLengths = []int64{
		utility.CopiedBlockMaxSize / 2,
		utility.CopiedBlockMaxSize,
		utility.CopiedBlockMaxSize * 2,
		utility.CopiedBlockMaxSize * 2.5,
	}

	for _, dataLength := range testDataLengths {
		currentData := make([]byte, dataLength)
		rand.Read(currentData)
		currentReader := bytes.NewReader(currentData)
		currentBuffer := new(bytes.Buffer)
		readLength, err := utility.FastCopy(currentBuffer, currentReader)
		assert.Equal(t, dataLength, readLength)
		assert.NoError(t, err)
		assert.Equal(t, currentData, currentBuffer.Bytes())
	}
}

func TestFastCopy_NotFails_OnEmptyData(t *testing.T) {
	emptyData := make([]byte, 0)
	reader := bytes.NewReader(emptyData)
	buffer := new(bytes.Buffer)
	readLength, err := utility.FastCopy(buffer, reader)
	result := buffer.Bytes()
	assert.Equal(t, int64(0), readLength)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFastCopy_ReturnsError_WhenReaderFails(t *testing.T) {
	reader := new(testtools.ErrorReader)
	buffer := new(bytes.Buffer)
	_, err := utility.FastCopy(buffer, reader)
	assert.Error(t, err)
}

func TestFastCopy_ReturnsError_WhenWriterFails(t *testing.T) {
	reader := strings.NewReader("data")
	writer := new(testtools.ErrorWriter)
	_, err := utility.FastCopy(writer, reader)
	assert.Error(t, err)
}

func TestSelectMatchingFiles_EmptyMask(t *testing.T) {
	files := map[string]bool{
		"/a":   true,
		"/b/c": true,
		"d":    true,
	}
	selected, err := utility.SelectMatchingFiles("", files)
	assert.NoError(t, err)
	assert.Equal(t, files, selected)
}

func TestSelectMatchingFiles_InvalidMask(t *testing.T) {
	files := map[string]bool{
		"/a":   true,
		"/b/c": true,
		"d":    true,
	}
	_, err := utility.SelectMatchingFiles("[a-c", files)
	assert.Error(t, err)
}

func TestSelectMatchingFiles_ValidMask(t *testing.T) {
	files := map[string]bool{
		"/a":   true,
		"/b/c": true,
		"/b/e": true,
		"d":    true,
	}
	selected, err := utility.SelectMatchingFiles("b/*", files)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"/b/c": true,
		"/b/e": true,
	}, selected)
}

func TestSanitizePath_Sanitize(t *testing.T) {
	assert.Equal(t, "home", utility.SanitizePath("/home"))
}

func TestSanitizePath_LeaveSame(t *testing.T) {
	assert.Equal(t, "home", utility.SanitizePath("home"))
}

func TestNormalizePath_Normalize(t *testing.T) {
	assert.Equal(t, "home", utility.NormalizePath("home/"))
}

func TestNormalizePath_LeaveSame(t *testing.T) {
	assert.Equal(t, "home", utility.NormalizePath("home"))
}

func TestPathsEqual_SamePaths(t *testing.T) {
	assert.True(t, utility.PathsEqual("/home/ismirn0ff", "/home/ismirn0ff"))
}

func TestPathsEqual_NeedNormalization(t *testing.T) {
	assert.True(t, utility.PathsEqual("/home/ismirn0ff", "/home/ismirn0ff/"))
}

func TestPathsEqual_SubdirectoryDoesNotEquate(t *testing.T) {
	assert.False(t, utility.PathsEqual("/home/ismirn0ff", "/home/"))
	assert.False(t, utility.PathsEqual("/home/", "/home/ismirn0ff"))
}

func TestPathsEqual_RelativeDoesNotEqualAbsolute(t *testing.T) {
	assert.False(t, utility.PathsEqual("home/ismirn0ff", "/home/ismirn0ff"))
}

func TestIsInDirectory_SamePaths(t *testing.T) {
	assert.True(t, utility.IsInDirectory("/home/", "/home/"))
}

func TestIsInDirectory_NeedPathNormalization(t *testing.T) {
	assert.True(t, utility.IsInDirectory("/home", "/home/"))
}

func TestIsInDirectory_NeedDirectoryNormalization(t *testing.T) {
	assert.True(t, utility.IsInDirectory("/home", "/home/"))
}

func TestIsInDirectory_NeedBothNormalization(t *testing.T) {
	assert.True(t, utility.IsInDirectory("/home/", "/home/"))
}

func TestIsInDirectory_IsSubdirectory(t *testing.T) {
	assert.True(t, utility.IsInDirectory("/home/ismirn0ff", "/home/"))
}

func TestIsInDirectory_IsDirectoryAbove(t *testing.T) {
	assert.False(t, utility.IsInDirectory("/home", "/home/ismirn0ff"))
}

func TestIsInDirectory_DifferentDirectories(t *testing.T) {
	assert.False(t, utility.IsInDirectory("/tmp", "/home/ismirn0ff"))
}

func TestGetSubdirectoryRelativePath_NormalizedDirectory(t *testing.T) {
	assert.Equal(t, "ismirn0ff/documents", utility.GetSubdirectoryRelativePath("/home/ismirn0ff/documents", "/home"))
}

func TestGetSubdirectoryRelativePath_NotNormalizedDirectory(t *testing.T) {
	assert.Equal(t, "ismirn0ff/documents", utility.GetSubdirectoryRelativePath("/home/ismirn0ff/documents/", "/home/"))
}
