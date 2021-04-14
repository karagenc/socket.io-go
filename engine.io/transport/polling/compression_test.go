package polling

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NYTimes/gziphandler"
	"github.com/stretchr/testify/assert"
)

func TestGzip(t *testing.T) {
	h := http.HandlerFunc(loremIpsumHandler)
	gh, err := gziphandler.NewGzipLevelAndMinSize(gzip.DefaultCompression, 0 /* gzip everything */)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip")

	gh(h).ServeHTTP(rec, req)

	resp := rec.Result()
	defer func() {
		err = resp.Body.Close()
		assert.Nil(t, err, "resp.Body.Close() should not return error")
	}()

	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"), "Content-Encoding should be set to gzip")

	r, err := compressedReader(resp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = r.Close()
		assert.Nil(t, err, "r.Close() should not return error")
	}()

	if _, ok := r.(*gzip.Reader); !ok {
		t.Fatal("*gzip.Reader expected")
	}

	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, loremIpsum, body, "the returned gzipped response should match the lorem ipsum text")
}

var loremIpsum = []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Suspendisse placerat magna a vulputate lobortis. Vestibulum auctor sapien purus, sit amet blandit lectus semper id. Pellentesque aliquet, libero ac blandit consectetur, risus erat egestas lacus, et ullamcorper magna turpis id quam. Aenean ornare ex quis ante ullamcorper facilisis. Fusce pretium lacus in nunc viverra hendrerit. Mauris a leo leo. Ut tincidunt varius urna et mollis. Proin fermentum turpis at posuere condimentum. Aenean mollis varius orci eget suscipit. Integer luctus erat ligula, quis aliquet elit euismod et. Fusce sed leo ac est sollicitudin efficitur. Vestibulum sagittis augue non ex vestibulum sagittis. Aliquam erat volutpat. Donec non justo tortor. Aliquam cursus tristique nulla, et venenatis lacus vehicula ac. Nunc molestie nisi eros, sit amet porttitor enim ultricies at. Suspendisse suscipit sem eget diam bibendum, eu suscipit augue lobortis. Quisque ut nisi eget justo auctor viverra. Donec pellentesque nunc at velit ullamcorper, vehicula pharetra metus dignissim. Pellentesque sed sem egestas massa fringilla vehicula ut vel neque. Phasellus sed luctus lectus, quis luctus purus. Cras consequat mauris eu est rhoncus pellentesque ac non turpis. Proin ullamcorper vitae sem vitae ultrices. Integer scelerisque convallis tortor, vel pulvinar arcu aliquam at. Duis eu augue id felis laoreet tempor in a nibh. Suspendisse augue neque, egestas sed pellentesque cursus, sagittis sit amet lorem. Integer non tellus sem. Proin sapien augue, efficitur sed nibh eu, scelerisque bibendum leo.")

func loremIpsumHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write(loremIpsum)
}
