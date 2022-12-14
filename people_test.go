package idmatch

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var Signatures = []signatureWithRepo{
	{repo: "repo1", name: "Bob", email: "Bob@google.com", hash: "aaa",
		time: time.Now().AddDate(0, -6, 0).Truncate(time.Second).UTC()},
	{repo: "repo2", name: "Bob", email: "Bob@google.com", hash: "bbb",
		time: time.Now().AddDate(0, -18, 0).Truncate(time.Second).UTC()},
	{repo: "repo1", name: "Alice", email: "alice@google.com", hash: "ccc",
		time: time.Now().AddDate(0, -15, 0).Truncate(time.Second).UTC()},
	{repo: "repo1", name: "Bob", email: "Bob@google.com", hash: "ddd",
		time: time.Now().AddDate(0, -2, 0).Truncate(time.Second).UTC()},
	{repo: "repo1", name: "Bob", email: "bad-email@domen", hash: "eee",
		time: time.Now().AddDate(0, -20, 0).Truncate(time.Second).UTC()},
	{repo: "repo1", name: "admin", email: "someone@google.com", hash: "fff",
		time: time.Now().AddDate(0, -4, 0).Truncate(time.Second).UTC()},
}

func TestPeopleNew(t *testing.T) {
	expected := People{
		1: {ID: 1, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"aaa", "repo1"}},
		2: {ID: 2, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"bbb", "repo2"}},
		3: {ID: 3, NamesWithRepos: []NameWithRepo{{"alice", ""}}, Emails: []string{"alice@google.com"},
			SampleCommit: &Commit{"ccc", "repo1"}},
		4: {ID: 4, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"ddd", "repo1"}},
	}
	people, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(t, err)
	require.Equal(t, expected, people)
}

func TestTwoPeopleMerge(t *testing.T) {
	require := require.New(t)
	people, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(err)
	mergedID, err := people.Merge(1, 2)
	expected := People{
		1: {ID: 1, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"}},
		3: {ID: 3, NamesWithRepos: []NameWithRepo{{"alice", ""}}, Emails: []string{"alice@google.com"},
			SampleCommit: &Commit{"ccc", "repo1"}},
		4: {ID: 4, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"ddd", "repo1"}},
	}
	require.Equal(int64(1), mergedID)
	require.Equal(expected, people)
	require.NoError(err)

	mergedID, err = people.Merge(3, 4)
	expected = People{
		1: {ID: 1, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"}},
		3: {ID: 3,
			NamesWithRepos: []NameWithRepo{{"alice", ""}, {"bob", ""}},
			Emails:         []string{"alice@google.com", "bob@google.com"}},
	}
	require.Equal(int64(3), mergedID)
	require.Equal(expected, people)
	require.NoError(err)

	mergedID, err = people.Merge(1, 3)
	expected = People{
		1: {ID: 1,
			NamesWithRepos: []NameWithRepo{{"alice", ""}, {"bob", ""}},
			Emails:         []string{"alice@google.com", "bob@google.com"}},
	}
	require.Equal(int64(1), mergedID)
	require.Equal(expected, people)
	require.NoError(err)
}

func TestFourPeopleMerge(t *testing.T) {
	people, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(t, err)
	mergedID, err := people.Merge(1, 2, 3, 4)
	expected := People{
		1: {ID: 1,
			NamesWithRepos: []NameWithRepo{{"alice", ""}, {"bob", ""}},
			Emails:         []string{"alice@google.com", "bob@google.com"}},
	}
	require.Equal(t, int64(1), mergedID)
	require.Equal(t, expected, people)
	require.NoError(t, err)
}

func TestDifferentExternalIdsMerge(t *testing.T) {
	people, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(t, err)
	people[1].ExternalID = "id1"
	people[2].ExternalID = "id2"
	_, err = people.Merge(1, 2)
	require.Error(t, err)
}

func TestPeopleForEach(t *testing.T) {
	people, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(t, err)
	var keys = make([]int64, 0, len(people))
	people.ForEach(func(key int64, val *Person) bool {
		keys = append(keys, key)
		return false
	})
	require.Equal(t, []int64{1, 2, 3, 4}, keys)
}

func tempFile(t *testing.T, pattern string) (*os.File, func()) {
	t.Helper()
	f, err := ioutil.TempFile("", pattern)
	require.NoError(t, err)
	return f, func() {
		require.NoError(t, f.Close())
		require.NoError(t, os.Remove(f.Name()))
	}
}

func TestFindSignatures(t *testing.T) {
	req := require.New(t)
	peopleFile, cleanup := tempFile(t, "*.csv")
	defer cleanup()

	err := storeSignaturesOnDisk(peopleFile.Name(), Signatures)
	req.NoError(err)
	people, err := findSignatures(context.TODO(), "0.0.0.0:3306", peopleFile.Name())
	req.NoError(err)
	req.Equal([]signatureWithRepo{
		{repo: "repo1", name: "bob", email: "bob@google.com", hash: "aaa", time: Signatures[0].time},
		{repo: "repo2", name: "bob", email: "bob@google.com", hash: "bbb", time: Signatures[1].time},
		{repo: "repo1", name: "alice", email: "alice@google.com", hash: "ccc", time: Signatures[2].time},
		{repo: "repo1", name: "bob", email: "bob@google.com", hash: "ddd", time: Signatures[3].time},
		{repo: "repo1", name: "bob", email: "bad-email@domen", hash: "eee", time: Signatures[4].time},
		{repo: "repo1", name: "admin", email: "someone@google.com", hash: "fff", time: Signatures[5].time},
	}, people)
}

func TestFindPeople(t *testing.T) {
	peopleFile, cleanup := tempFile(t, "*.csv")
	defer cleanup()

	err := storeSignaturesOnDisk(peopleFile.Name(), Signatures)
	if err != nil {
		return
	}
	people, nameFreqs, emailFreqs, err := FindPeople(
		context.TODO(), "0.0.0.0:3306", peopleFile.Name(), newTestBlacklist(t), 12)
	if err != nil {
		return
	}
	expected := People{
		1: {ID: 1, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"aaa", "repo1"}},
		2: {ID: 2, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"bbb", "repo2"}},
		3: {ID: 3, NamesWithRepos: []NameWithRepo{{"alice", ""}}, Emails: []string{"alice@google.com"},
			SampleCommit: &Commit{"ccc", "repo1"}},
		4: {ID: 4, NamesWithRepos: []NameWithRepo{{"bob", ""}}, Emails: []string{"bob@google.com"},
			SampleCommit: &Commit{"ddd", "repo1"}},
	}
	require.Equal(t, expected, people)
	require.Equal(t, map[string]*Frequency{"alice": {0, 1},
		"admin": {1, 1}, "bob": {2, 4}}, nameFreqs)
	require.Equal(t, map[string]*Frequency{"bob@google.com": {2, 3},
		"alice@google.com": {0, 1}, "bad-email@domen": {0, 1},
		"someone@google.com": {1, 1}}, emailFreqs)
}

func TestReadPeopleFromDatabase(t *testing.T) {
	// TODO(zurk): write this test
}

func TestStoreAndReadPeopleOnDisk(t *testing.T) {
	req := require.New(t)
	peopleFile, cleanup := tempFile(t, "*.csv")
	defer cleanup()

	err := storeSignaturesOnDisk(peopleFile.Name(), Signatures)
	req.NoError(err)
	peopleFileContent, err := ioutil.ReadFile(peopleFile.Name())
	req.NoError(err)
	expectedContent := `repo,name,email,hash,time
repo1,Bob,Bob@google.com,aaa,` + Signatures[0].time.Format(time.RFC3339) + `
repo2,Bob,Bob@google.com,bbb,` + Signatures[1].time.Format(time.RFC3339) + `
repo1,Alice,alice@google.com,ccc,` + Signatures[2].time.Format(time.RFC3339) + `
repo1,Bob,Bob@google.com,ddd,` + Signatures[3].time.Format(time.RFC3339) + `
repo1,Bob,bad-email@domen,eee,` + Signatures[4].time.Format(time.RFC3339) + `
repo1,admin,someone@google.com,fff,` + Signatures[5].time.Format(time.RFC3339) + `
`
	req.Equal(expectedContent, string(peopleFileContent))

	commitsRead, err := readSignaturesFromDisk(peopleFile.Name())
	req.NoError(err)
	expectedPersonsRead := []signatureWithRepo{
		0: {repo: "repo1", name: "bob", email: "bob@google.com", hash: "aaa", time: Signatures[0].time},
		1: {repo: "repo2", name: "bob", email: "bob@google.com", hash: "bbb", time: Signatures[1].time},
		2: {repo: "repo1", name: "alice", email: "alice@google.com", hash: "ccc", time: Signatures[2].time},
		3: {repo: "repo1", name: "bob", email: "bob@google.com", hash: "ddd", time: Signatures[3].time},
		4: {repo: "repo1", name: "bob", email: "bad-email@domen", hash: "eee", time: Signatures[4].time},
		5: {repo: "repo1", name: "admin", email: "someone@google.com", hash: "fff", time: Signatures[5].time},
	}
	req.Equal(expectedPersonsRead, commitsRead)
}

func TestWriteAndReadParquet(t *testing.T) {
	tmpfile, cleanup := tempFile(t, "*.parquet")
	defer cleanup()

	expectedPeople, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(t, err)
	for _, p := range expectedPeople {
		p.SampleCommit = nil
	}

	err = expectedPeople.WriteToParquet(tmpfile.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	people, provider, err := readFromParquet(tmpfile.Name())
	require.Equal(t, expectedPeople, people)
	require.Equal(t, "", provider)
}

func TestWriteAndReadParquetWithExternalID(t *testing.T) {
	tmpfile, cleanup := tempFile(t, "*.parquet")
	defer cleanup()

	expectedPeople, err := newPeople(Signatures, newTestBlacklist(t))
	require.NoError(t, err)
	for _, p := range expectedPeople {
		p.SampleCommit = nil
	}

	expectedIDProvider := "test"
	expectedPeople[1].ExternalID = "username1"
	expectedPeople[2].ExternalID = "username2"

	err = expectedPeople.WriteToParquet(tmpfile.Name(), expectedIDProvider)
	require.NoError(t, err)
	people, provider, err := readFromParquet(tmpfile.Name())
	require.Equal(t, expectedPeople, people)
	require.Equal(t, expectedIDProvider, provider)
}

func TestCleanName(t *testing.T) {
	require := require.New(t)
	for _, names := range [][]string{
		{"  name", "name"},
		{"name  	name  ", "name name"},
		{"name  	name\nsurname", "name name surname"},
		{"name???name", "name name"}, // special space %u3000
	} {
		cName, err := cleanName(names[0])
		require.NoError(err)
		require.Equal(names[1], cName)
	}
}

func TestRemoveParens(t *testing.T) {
	require := require.New(t)
	require.Equal("something something2", removeParens("something (delete it) something2"))
	require.Equal("something () something2", removeParens("something () something2"))
	require.Equal("something (2) something2", removeParens("something (1) (2) something2"))
	require.Equal("something(nospace)something2", removeParens("something(nospace)something2"))
}

func TestNormalizeSpaces(t *testing.T) {
	require := require.New(t)
	require.Equal("1 2", normalizeSpaces("1 2"))
	require.Equal("1 2", normalizeSpaces("1  \t  2 \n\n"))
	require.Equal("12", normalizeSpaces("12"))
}

func TestCountFreqs(t *testing.T) {
	freqs, err := countFreqs(Signatures, func(c signatureWithRepo) string { return c.name },
		cleanName, time.Now().AddDate(0, -19, 0))
	require.NoError(t, err)
	require.Equal(t, map[string]*Frequency{"alice": {1, 1}, "admin": {1, 1}, "bob": {3, 4}}, freqs)
}

func TestGetStats(t *testing.T) {
	nameFreqs, emailFreqs, err := getStats(Signatures, time.Now().AddDate(0, -12, 0))
	require.NoError(t, err)
	require.Equal(t, map[string]*Frequency{"alice": {0, 1}, "admin": {1, 1}, "bob": {2, 4}},
		nameFreqs)
	require.Equal(t, map[string]*Frequency{"bob@google.com": {2, 3},
		"alice@google.com": {0, 1}, "bad-email@domen": {0, 1},
		"someone@google.com": {1, 1}}, emailFreqs)
}
