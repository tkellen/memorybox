package memorybox_test

/*
type testIO struct {
	reader *bufpipe.PipeReader
	writer *bufpipe.PipeWriter
}

func TestMetaGet(t *testing.T) {
	type testCase struct {
		ctx           context.Context
		store         memorybox.Store
		io            *testIO
		fixtures      []*archive.File
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	contents := [][]byte{
		[]byte("foo-content"),
		[]byte("bar-content"),
	}
	var fixtures []*archive.File
	for _, content := range contents {
		fixture, err := archive.NewSha256("fixture", filebuffer.New(content))
		if err != nil {
			t.Fatalf("test setup: %s", err)
		}
		fixtures = append(fixtures, fixture)
	}
	table := map[string]testCase{
		"request existing metafile": {
			ctx:   context.Background(),
			store: testingstore.New(fixtures),
			io: func() *testIO {
				reader, writer := bufpipe.New(nil)
				return &testIO{
					reader: reader,
					writer: writer,
				}
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name(), // first file is data object
			expectedBytes: contents[1],        // second file is metafile
			expectedErr:   nil,
		},
		"request existing metafile with failed copy to sink": {
			ctx:   context.Background(),
			store: testingstore.New(fixtures),
			io: func() *testIO {
				reader, writer := bufpipe.New(nil)
				reader.Close()
				return &testIO{
					reader: reader,
					writer: writer,
				}
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name(), // first file is data object
			expectedBytes: contents[1],        // second file is metafile
			expectedErr:   errors.New("closed pipe"),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			err := memorybox.MetaGet(test.ctx, test.store, test.request, test.io.writer)
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			test.io.writer.Close()
			if err != nil && test.expectedErr != nil && !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
			if err == nil && test.expectedBytes != nil {
				actualBytes, _ := ioutil.ReadAll(test.io.reader)
				if diff := cmp.Diff(test.expectedBytes, actualBytes); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestMetaSetAndDelete(t *testing.T) {
	ctx := context.Background()
	contents := [][]byte{
		[]byte("foo-content"),
		[]byte("bar-content"),
	}
	var fixtures []*archive.File
	for _, content := range contents {
		fixture, err := archive.NewSha256("fixture", filebuffer.New(content))
		if err != nil {
			t.Fatalf("test setup: %s", err)
		}
		fixtures = append(fixtures, fixture)
	}
	testStore := testingstore.New(fixtures)
	request := fixtures[0].Name()
	expectedKeyAndValue := "test"
	// add meta key
	if err := memorybox.MetaSet(ctx, testStore, request, expectedKeyAndValue, expectedKeyAndValue); err != nil {
		t.Fatal(err)
	}
	// confirm key was set by asking for the metafile again
	reader, writer := bufpipe.New(nil)
	if err := memorybox.MetaGet(ctx, testStore, request, writer); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	metaSetCheck, metaSetCheckErr := archive.NewSha256(ctx, reader)
	if metaSetCheckErr != nil {
		t.Fatal(metaSetCheckErr)
	}
	if expectedKeyAndValue != metaSetCheck.MetaGet(expectedKeyAndValue) {
		t.Fatal("expected key %[1] to be set to %[1], saw %[1]", metaSetCheck.MetaGet(expectedKeyAndValue))
	}
	// remove key
	if err := memorybox.MetaDelete(ctx, testStore, request, expectedKeyAndValue); err != nil {
		t.Fatal(err)
	}
	// confirm key was removed by asking for it again
	reader, writer = bufpipe.New(nil)
	if err := memorybox.MetaGet(ctx, testStore, request, writer); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	metaDeleteCheck, metaDeleteCheckErr := archive.NewFromReader(ctx, archive.Sha256, reader)
	if metaDeleteCheckErr != nil {
		t.Fatal(metaDeleteCheckErr)
	}
	if metaDeleteCheck.MetaGet(expectedKeyAndValue) != nil {
		t.Fatalf("expected key %s to be deleted", expectedKeyAndValue)
	}
}

func TestMetaFailures(t *testing.T) {
	type testCase struct {
		ctx           context.Context
		store         memorybox.Store
		fixtures      []testingstore.Fixture
		request       string
		expectedBytes []byte
		expectedErr   error
	}
	fixtures := []testingstore.Fixture{
		testingstore.NewFixture("something", false, archive.Sha256),
		testingstore.NewFixture("something", true, archive.Sha256),
	}
	table := map[string]testCase{
		"request missing metafile": {
			ctx:           context.Background(),
			store:         testingstore.New(fixtures),
			fixtures:      fixtures,
			request:       "missing",
			expectedBytes: nil,
			expectedErr:   errors.New("0 objects"),
		},
		"request with failed search": {
			ctx: context.Background(),
			store: func() memorybox.Store {
				store := testingstore.New(fixtures)
				store.SearchErrorWith = errors.New("bad search")
				return store
			}(),
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad search"),
		},
		"request existing metafile with failed retrieval": {
			ctx: context.Background(),
			store: func() memorybox.Store {
				store := testingstore.New(fixtures)
				store.GetErrorWith = errors.New("bad get")
				return store
			}(),
			fixtures:      fixtures,
			request:       fixtures[0].Name,
			expectedBytes: nil,
			expectedErr:   errors.New("bad get"),
		},
	}
	for name, test := range table {
		test := test
		t.Run("Meta "+name, func(t *testing.T) {
			err := memorybox.MetaGet(test.ctx, test.store, test.request, bytes.NewBuffer([]byte{}))
			if err == nil {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
		})
		t.Run("MetaSet "+name, func(t *testing.T) {
			err := memorybox.MetaSet(test.ctx, test.store, test.request, "test", "test")
			if err == nil {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
		})
		t.Run("MetaDelete "+name, func(t *testing.T) {
			err := memorybox.MetaDelete(test.ctx, test.store, test.request, "test")
			if err == nil {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), test.expectedErr.Error()) {
				t.Fatalf("expected error: %s, got %s", test.expectedErr, err.Error())
			}
		})
	}
}
*/
