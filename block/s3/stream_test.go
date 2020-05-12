package s3_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/s3"
	s3a "github.com/treeverse/lakefs/block/s3"
)

func TestS3StreamingReader_Read(t *testing.T) {
	// generate test data
	cases := []struct {
		Name      string
		Input     []byte
		ChunkSize int
		Expected  []byte
	}{
		{
			Name:      "chunk5_data10",
			Input:     mustReadFile(t, "testdata/chunk5_data10.input"),
			ChunkSize: 5,
			Expected:  mustReadFile(t, "testdata/chunk5_data10.output"),
		},
		{
			Name:      "chunk250_data500",
			Input:     mustReadFile(t, "testdata/chunk250_data500.input"),
			ChunkSize: 250,
			Expected:  mustReadFile(t, "testdata/chunk250_data500.output"),
		},
		{
			Name:      "chunk250_data510",
			Input:     mustReadFile(t, "testdata/chunk250_data510.input"),
			ChunkSize: 250,
			Expected:  mustReadFile(t, "testdata/chunk250_data510.output"),
		},
		{
			Name:      "chunk600_data240",
			Input:     mustReadFile(t, "testdata/chunk600_data240.input"),
			ChunkSize: 250,
			Expected:  mustReadFile(t, "testdata/chunk600_data240.output"),
		},
		{
			Name:      "chunk3000_data10",
			Input:     mustReadFile(t, "testdata/chunk3000_data10.input"),
			ChunkSize: 250,
			Expected:  mustReadFile(t, "testdata/chunk3000_data10.output"),
		},
		{
			Name:      "chunk5_data0",
			Input:     mustReadFile(t, "testdata/chunk5_data0.input"),
			ChunkSize: 5,
			Expected:  mustReadFile(t, "testdata/chunk5_data0.output"),
		},
	}

	for _, cas := range cases {
		t.Run(cas.Name, func(t *testing.T) {
			// this is just boilerplate to create a signature
			contentLength := s3a.CalculateStreamSizeForPayload(int64(len(cas.Input)), cas.ChunkSize)
			creds := credentials.NewStaticCredentials("AKIAJIEMTME6UEVWXB2Q", "vlJMuY24GyMRXLca7+V2Xc6IEEAyZTnZ29NJsspN", "")
			sigTime, _ := time.Parse("Jan 2 15:04:05 2006 -0700", "Apr 7 15:13:13 2005 -0700")
			req, _ := http.NewRequest(http.MethodPut, "https://s3.amazonaws.com/example/foo", nil)
			req.Header.Set("Content-Encoding", "aws-chunked")
			req.Header.Set("x-amz-content-sha", fmt.Sprintf("STREAMING-AWS4-HMAC-SHA256-PAYLOAD"))
			req.Header.Set("x-amz-decoded-content-length", fmt.Sprintf("%d", len(cas.Input)))
			req.Header.Set("Expect", "100-Continue")
			req.ContentLength = int64(contentLength)
			baseSigner := v4.NewSigner(creds)

			signature, err := baseSigner.Sign(req, nil, s3.ServiceName, "us-east-1", sigTime)
			if err != nil {
				t.Fatal(err)
			}

			for k := range signature {
				req.Header.Set(k, signature.Get(k))
			}

			sigSeed, err := v4.GetSignedRequestSignature(req)
			if err != nil {
				t.Fatal(err)
			}

			data := &s3a.StreamingReader{
				Reader:       ioutil.NopCloser(bytes.NewBuffer(cas.Input)),
				Size:         len(cas.Input),
				StreamSigner: v4.NewStreamSigner("us-east-1", s3.ServiceName, sigSeed, creds),
				Time:         sigTime,
				ChunkSize:    cas.ChunkSize,
			}

			out, err := ioutil.ReadAll(data)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(out, cas.Expected) {
				t.Fatalf("got wrong chunked data")
			}

			if int64(len(cas.Expected)) != contentLength {
				t.Fatalf("content length is wrong, got %d, expected %d", contentLength, len(cas.Expected))
			}
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}