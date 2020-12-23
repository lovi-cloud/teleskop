package metadata

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/lovi-cloud/teleskop/metadata/team"

	yaml "gopkg.in/yaml.v2"

	pb "github.com/whywaita/satelit/api/satelit_datastore"
)

// Server is
type Server struct {
	client pb.SatelitDatastoreClient

	supervisorEndpoint string
	supervisorToken    string
}

const secretHash = "398c4de5b9e71ef42b7dca9e4d0d1b661ae85c4996f8d73fb015e3ae6b08a978222784bf348baf6c8e7c96a6d1a2886d"

// New is
func New(client pb.SatelitDatastoreClient, endpoint, token string) *Server {
	return &Server{
		client:             client,
		supervisorEndpoint: endpoint,
		supervisorToken:    token,
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
	mux.Handle("/teamid", s.loggingHandler(s.teamIDHandler()))
	mux.Handle(fmt.Sprintf("/s/endpoint/%s", secretHash), s.loggingHandler(s.supervisorEndpointHandler()))
	mux.Handle(fmt.Sprintf("/s/token/%s", secretHash), s.loggingHandler(s.supervisorTokenHandler()))

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

		var userKeys []string

		if strings.HasSuffix(addr.IP.String(), "104") {
			// 104 is bench server, not set isucon keys
			userKeys = []string{}
		} else {
			resp, err := s.client.GetISUCONUserKeys(r.Context(), &pb.GetISUCONUserKeysRequest{
				Address: addr.IP.String(),
			})
			if err != nil {
				msg := fmt.Sprintf("failed to get ISUCON user keys: %+v", err)
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(msg))
				return
			}

			userKeys = resp.Keys
		}

		adminKeys, err := s.client.GetISUCONAdminKeys(r.Context(), &pb.GetISUCONAdminKeysRequest{})
		if err != nil {
			msg := fmt.Sprintf("failed to get ISUCON admin keys: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}

		teamAddrs := generateTeamAddrs(addr.IP)

		config := config{
			ManageEtcHosts: false,
			Hostname:       nameResp.Hostname,
			FQDN:           nameResp.Hostname,
			Users: []user{
				{
					Name:              "isucon",
					Sudo:              "ALL=(ALL) NOPASSWD:ALL",
					Groups:            "users, admin",
					Chpasswd:          "{ expire: False }",
					LockPasswd:        false,
					SSHAuthorizedKeys: userKeys,
				},
				{
					Name:              "isucon-admin",
					Sudo:              "ALL=(ALL) NOPASSWD:ALL",
					Groups:            "users, admin",
					SSHAuthorizedKeys: adminKeys.Keys,
				},
				{
					Name:       "cycloud",
					Sudo:       "ALL=(ALL) NOPASSWD:ALL",
					Groups:     "users, admin",
					Passwd:     "$6$VmEK.acZVCJ$7dxqbv.7f/Eyh6jIXeM5Ns2R8vqtfRLWhRiJXL0EcnubkQlf2F5EyOldDyN5s1zBz6ubNDtSSEvq.VnmlTCoC.",
					Chpasswd:   "{ expire: False }",
					LockPasswd: false,
				},
			},
			BootCMD: []string{
				strings.Join([]string{"cloud-init-per", "once", "update-etc-hosts-0", "echo", teamAddrs[0], "isu1.t.isucon.dev", ">>", "/etc/hosts"}, " "),
				strings.Join([]string{"cloud-init-per", "once", "update-etc-hosts-1", "echo", teamAddrs[1], "isu2.t.isucon.dev", ">>", "/etc/hosts"}, " "),
				strings.Join([]string{"cloud-init-per", "once", "update-etc-hosts-2", "echo", teamAddrs[2], "isu3.t.isucon.dev", ">>", "/etc/hosts"}, " "),
				strings.Join([]string{"cloud-init-per", "once", "update-etc-hosts-3", "echo", teamAddrs[3], "isubench.t.isucon.dev", ">>", "/etc/hosts"}, " "),
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

func (s *Server) teamIDHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			msg := fmt.Sprintf("failed to parse request address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}

		teamID, err := team.GetTeamID(addr.IP.String())
		if err != nil {
			msg := fmt.Sprintf("failed to get teamID: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}
		w.Write([]byte(fmt.Sprintf("%d", teamID)))
		return
	})
}

func (s *Server) supervisorEndpointHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			msg := fmt.Sprintf("failed to parse request address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}

		if !strings.HasSuffix(addr.IP.String(), "104") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Write([]byte(s.supervisorEndpoint))
		return
	})
}

func (s *Server) supervisorTokenHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			msg := fmt.Sprintf("failed to parse request address: %+v", err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(msg))
			return
		}

		if !strings.HasSuffix(addr.IP.String(), "104") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Write([]byte(s.supervisorToken))
		return
	})
}

func generateTeamAddrs(addr net.IP) []string {
	prefix := strings.Split(addr.String(), ".")[:3]
	result := []string{}
	for i := 101; i < 105; i++ {
		tmp := append(prefix, fmt.Sprintf("%d", i))
		result = append(result, strings.Join(tmp, "."))
	}
	return result
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
