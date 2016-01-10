package main

import (
	"./api"
	"compress/gzip"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"reflect"
	"strings"
)

// MaybeExtractQ extracts proto from HTTP request and returns it.
// Nil indicates failure, and appropriate status / description is written
// to response.
func MaybeExtractQ(w http.ResponseWriter, r *http.Request, defaultQ proto.Message) *proto.Message {
	q := proto.Clone(defaultQ)
	if r.Method == "POST" {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, "Failed to read POST body %v", err)
			return nil
		}
		err = proto.Unmarshal(reqBody, q)
		if err != nil {
			fmt.Fprintf(w, "Failed to parse POST body as binary proto: %v", err)
			return nil
		}
	} else {
		err := r.ParseForm()
		if err != nil {
			http.NotFound(w, r)
			fmt.Fprintf(w, "strange query %v", err)
			return nil
		}
		pb := r.Form.Get("pb")
		if pb == "" {
			http.NotFound(w, r)
			fmt.Fprintf(w, "Non-empty jsonpb-encoded pb param required for GET")
			return nil
		}
		err = jsonpb.UnmarshalString(pb, q)
		if err != nil {
			fmt.Fprintf(w, "Failed to parse pb param %v", err)
			return nil
		}
	}
	return &q
}

func WriteS(
	w http.ResponseWriter, r *http.Request, s proto.Message, e error) {
	if e != nil {
		http.NotFound(w, r)
		fmt.Fprintf(w, "internal error: %v", e)
		return
	}
	data, err := proto.Marshal(s)
	if err != nil {
		w.Header().Set("Content-Type", "text/json")
		fmt.Fprintf(w, "{\"error\": \"Internal error: failed to encode return pb\"}")
	} else {
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Write(data)
	}
}

// JsonpbHandler converts given grpc server method of type
// func(context.Context, *<RequestMessage>) (*<ResponseMessage>, error)
// to a wrapped HTTP handler.
// If the method doesn't match this type, JsonpbHandler will panic.
func JsonpbHandler(grpcServerMethod interface{}) http.HandlerFunc {
	mType := reflect.TypeOf(grpcServerMethod)
	if mType.Kind() != reflect.Func || mType.NumIn() != 2 || mType.NumOut() != 2 {
		log.Panicf("Expecting func(2 args) (2 args), got %v", mType)
	}
	reqType := mType.In(1).Elem()
	return func(w http.ResponseWriter, r *http.Request) {
		q := MaybeExtractQ(w, r, reflect.New(reqType).Interface().(proto.Message))
		if q != nil {
			log.Printf("Request: %#v", *q)
			retVals := reflect.ValueOf(grpcServerMethod).Call([]reflect.Value{
				reflect.ValueOf(context.Background()),
				reflect.ValueOf(*q),
			})
			// Can't cast nil to error.
			var err error
			if retVals[1].Interface() != nil {
				err = retVals[1].Interface().(error)
			}
			WriteS(w, r, retVals[0].Interface().(proto.Message), err)
		}
	}
}

// Adopted from https://gist.github.com/the42/1956518
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Wrap given handler and support gzip compression
// (used when allowed by browsers).
func GzipHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gzr := gzipResponseWriter{Writer: gz, ResponseWriter: w}
			h(gzr, r)
		} else {
			h(w, r)
		}
	}
}

func FileServingHandler(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/root/bonsai/static/"+filename)
	}
}

// Return "prod" or "staging" (or other env type).
// This function might break when kubernetes spec changes.
// When validation fails, it returns "".
func GetEnvType() string {
	val, ok := os.LookupEnv("HOSTNAME")
	if !ok {
		return ""
	}
	tokens := strings.Split(val, "-")
	if len(tokens) < 3 {
		return ""
	}
	if tokens[0] != "bonsai" || tokens[2] != "frontend" {
		return ""
	}
	return tokens[1]
}

func main() {
	const port = 8000
	log.Printf("Starting frontend server http://localhost:%d\n", port)
	log.Printf("Env: %#v", os.Environ())
	envType := GetEnvType()
	if envType == "" {
		log.Panic("Failed to extract env type from HOSTNAME. Crashing")
	}
	log.Printf("EnvType: %s", envType)
	mime.AddExtensionType(".svg", "image/svg+xml")

	// Enforce that NewFeService implements the service defined in proto.
	var fe api.FrontendServiceServer
	fe = NewFeService(envType)

	// Dispatchers.
	http.HandleFunc("/api/debug", GzipHandler(JsonpbHandler(fe.Debug)))
	http.HandleFunc("/api/biospheres", GzipHandler(JsonpbHandler(fe.Biospheres)))
	http.HandleFunc("/api/add_biosphere", GzipHandler(JsonpbHandler(fe.AddBiosphere)))
	http.HandleFunc("/api/change_exec", JsonpbHandler(fe.ChangeExec))
	http.HandleFunc("/api/biosphere_frames", GzipHandler(JsonpbHandler(fe.BiosphereFrames)))
	// Static files.
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("/root/bonsai/static"))))
	// Special parmalinks.
	http.HandleFunc("/", FileServingHandler("landing.html"))
	http.HandleFunc("/biosphere/", FileServingHandler("biosphere.html"))
	http.HandleFunc("/debug", FileServingHandler("debug.html"))

	// Start FE server and block on it forever.
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
