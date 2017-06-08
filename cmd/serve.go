package cmd

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/eamon-collins/goptometry/pkg/ui/data/swagger"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/philips/go-bindata-assetfs"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/eamon-collins/goptometry/proto"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Launches the example webserver on https://localhost:10000",
	Run: func(cmd *cobra.Command, args []string) {
		serve()
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
}

type myService struct{}

func (m *myService) Echo(c context.Context, s *pb.EchoMessage) (*pb.EchoMessage, error) {
	fmt.Printf("rpc request Echo(%q)\n", s.Value)
	return s, nil
}

func (m *myService) Index(c context.Context, s *pb.IndexRequest) (*pb.IndexResponse, error) {
	fmt.Printf("Accessing Index\n")
	html := "this is a test phrase"
	return &pb.IndexResponse{html}, nil
}

func newServer() *myService {
	return new(myService)
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise. Copied from cockroachdb.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

func serveSwagger(mux *http.ServeMux) {
	mime.AddExtensionType(".svg", "image/svg+xml")

	// Expose files in third_party/swagger-ui/ on <host>/swagger-ui
	fileServer := http.FileServer(&assetfs.AssetFS{
		Asset:    swagger.Asset,
		AssetDir: swagger.AssetDir,
		Prefix:   "third_party/swagger-ui",
	})
	prefix := "/swagger-ui/"
	mux.Handle(prefix, http.StripPrefix(prefix, fileServer))
}

func serve() {
	opts := []grpc.ServerOption{
		grpc.Creds(credentials.NewClientTLSFromCert(demoCertPool, "localhost:10000"))}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterIndexServiceServer(grpcServer, newServer())
	ctx := context.Background()

	dcreds := credentials.NewTLS(&tls.Config{
		ServerName: demoAddr,
		RootCAs:    demoCertPool,
	})
	dopts := []grpc.DialOption{grpc.WithTransportCredentials(dcreds)}

	mux := http.NewServeMux()
	mux.HandleFunc("/swagger.json", func(w http.ResponseWriter, req *http.Request) {
		io.Copy(w, strings.NewReader(pb.Swagger))
	})

	//look into other ways of having this, at the moment not really using proto buffer at all
	gwmux := runtime.NewServeMux()
	err := pb.RegisterIndexServiceHandlerFromEndpoint(ctx, gwmux, demoAddr, dopts)
	if err != nil {
		fmt.Printf("serve: %v\n", err)
		return
	}

	fs := http.FileServer(http.Dir("../static"))
	mux.Handle("/", fs) //  "/", gwmux
	//serveSwagger(mux)


	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	srv := &http.Server{
		Addr:    demoAddr,
		Handler: grpcHandlerFunc(grpcServer, mux),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*demoKeyPair},
			NextProtos:   []string{"h2"},
		},
	}

	fmt.Printf("grpc on port: %d\n", port)
	err = srv.Serve(tls.NewListener(conn, srv.TLSConfig))

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

	return
}
