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

	pb "github.com/whywaita/satelit/api/satelit_datastore"
)

// Server is
type Server struct {
	client pb.SatelitDatastoreClient
}

// New is
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

		userKeys, err := s.client.GetISUCONUserKeys(r.Context(), &pb.GetISUCONUserKeysRequest{
			Address: addr.IP.String(),
		})
		if err != nil {
			msg := fmt.Sprintf("failed to get ISUCON user keys: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}
		adminKeys, err := s.client.GetISUCONAdminKeys(r.Context(), &pb.GetISUCONAdminKeysRequest{})
		if err != nil {
			msg := fmt.Sprintf("failed to get ISUCON admin keys: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}

		config := config{
			ManageEtcHosts: true,
			Hostname:       nameResp.Hostname,
			FQDN:           nameResp.Hostname,
			Users: []user{
				{
					Name:              "isucon",
					Sudo:              "ALL=(ALL) NOPASSWD:ALL",
					Groups:            "users, admin",
					SSHAuthorizedKeys: userKeys.Keys,
				},
				{
					Name:              "isucon-admin",
					Sudo:              "ALL=(ALL) NOPASSWD:ALL",
					Groups:            "users, admin",
					SSHAuthorizedKeys: adminKeys.Keys,
				},
			},
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
	Sudo              string   `yaml:"sudo,omitempty"`
	Groups            string   `yaml:"groups,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
}
