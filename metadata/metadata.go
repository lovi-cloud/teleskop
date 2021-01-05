package metadata

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"

	yaml "gopkg.in/yaml.v2"

	pb "github.com/lovi-cloud/satelit/api/satelit_datastore"
)

// Server is implement metadata server
type Server struct {
	client pb.SatelitDatastoreClient
}

// New create a instance of gRPC server
func New(client pb.SatelitDatastoreClient) *Server {
	return &Server{
		client: client,
	}
}

// Serve is
func (s *Server) Serve(ctx context.Context, addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", s.loggingHandler(http.NotFoundHandler()))
	mux.Handle("/meta-data", s.loggingHandler(s.metadataHandler()))
	mux.Handle("/user-data", s.loggingHandler(s.userdataHandler()))

	srv := http.Server{
		Handler: mux,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to shutdown: %+v", err)
		}
		close(idleConnsClosed)
	}()

	if err := srv.Serve(l); err != http.ErrServerClosed {
		return fmt.Errorf("failed to serve: %w", err)
	}

	<-idleConnsClosed
	return nil
}

func (s *Server) loggingHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		fmt.Printf("http request: url=%s, remote=%s, code=%d\n", r.URL.String(), r.RemoteAddr, rec.Code)
		w.WriteHeader(rec.Code)
		io.Copy(w, rec.Body)
	})
}

func (s *Server) metadataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			msg := fmt.Sprintf("failed to parse request remote address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(msg))
			return
		}
		resp, err := s.client.GetHostnameByAddress(r.Context(), &pb.GetHostnameByAddressRequest{
			Address: addr.IP.String(),
		})
		if err != nil {
			msg := fmt.Sprintf("failed to get hostname by address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}
		w.Write([]byte(fmt.Sprintf("hostname: %s\n", resp.Hostname)))
		return
	})
}

func (s *Server) userdataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			msg := fmt.Sprintf("failed to parse request remote address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(msg))
			return
		}

		nameResp, err := s.client.GetHostnameByAddress(r.Context(), &pb.GetHostnameByAddressRequest{
			Address: addr.IP.String(),
		})
		if err != nil {
			msg := fmt.Sprintf("failed to get hostname by address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}

		config := config{
			ManageEtcHosts: false,
			Hostname:       nameResp.Hostname,
			FQDN:           nameResp.Hostname,
		}

		out, err := yaml.Marshal(config)
		if err != nil {
			msg := fmt.Sprintf("failed to parse user-data: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}
		out = append([]byte("#cloud-config\n"), out...)
		w.Write(out)
	})
}

type config struct {
	ManageEtcHosts bool     `yaml:"manage_etc_hosts"`
	FQDN           string   `yaml:"fqdn,omitempty"`
	Hostname       string   `yaml:"hostname,omitempty"`
	Users          []user   `yaml:"users,omitempty"`
	BootCMD        []string `yaml:"bootcmd,omitempty"`
	RunCMD         []string `yaml:"runcmd,omitempty"`
}

type user struct {
	Name              string   `yaml:"name"`
	Passwd            string   `yaml:"passwd,omitempty"`
	Chpasswd          string   `yaml:"chpasswd,omitempty"`
	LockPasswd        bool     `yaml:"lock_passwd"`
	Sudo              string   `yaml:"sudo,omitempty"`
	Groups            string   `yaml:"groups,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
}
