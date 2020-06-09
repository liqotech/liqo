package mutate

import (
	"fmt"
	"html"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"log"
	"net/http"
	"os"
	"time"
)

type MutationConfig struct {
	SecretNamespace string
	SecretName      string
	CertFile        string
	KeyFile         string
}

type MutationServer struct {
	client *kubernetes.Clientset
	mux    *http.ServeMux
	server *http.Server

	config *MutationConfig
}

func NewMutationServer(c *MutationConfig) (*MutationServer, error) {
	var err error

	s := &MutationServer{}
	s.config = c

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/", handleRoot)
	s.mux.HandleFunc("/mutate", s.handleMutate)

	configPath := os.Getenv("KUBECONFIG")
	var config *rest.Config
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	if s.client, err = kubernetes.NewForConfig(config); err != nil {
		return nil, err
	}

	s.server = &http.Server{
		Addr:           ":8443",
		Handler:        s.mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1048576
	}

	return s, nil
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "hello %q", html.EscapeString(r.URL.Path))
}

func (s *MutationServer) handleMutate(w http.ResponseWriter, r *http.Request) {
	// read the body / request
	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		s.sendError(err, w)
		return
	}

	// mutate the request
	mutated, err := s.Mutate(body, true)
	if err != nil {
		s.sendError(err, w)
		return
	}

	// and write it back
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(mutated)
}

func (s *MutationServer) sendError(err error, w http.ResponseWriter) {
	log.Println(err)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "%s", err)
}

func (s *MutationServer) Serve() {
	secret, err := s.client.CoreV1().Secrets(s.config.SecretNamespace).Get(s.config.SecretName, metav1.GetOptions{})
	if err != nil {
		klog.Error("cannot get the pod mutator secret in namespace ", s.config.SecretNamespace)
		os.Exit(1)
	}

	writeSecret(secret, s.config.CertFile, s.config.KeyFile)

	log.Fatal(s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile))
}

func writeSecret(secret *corev1.Secret, certFile, keyFile string) {
	f, err := os.Create(certFile)
	if err != nil {
		klog.Error("cannot create the cert.pem file")
		os.Exit(1)
	}
	if n, err := f.Write(secret.Data["cert.pem"]); n != len(secret.Data["cert.pem"]) || err != nil {
		klog.Error("error in writing cert.pem")
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		klog.Error("error in closing cert.pem file")
		os.Exit(1)
	}

	f, err = os.Create(keyFile)
	if err != nil {
		klog.Error("cannot create the key.pem file")
		os.Exit(1)
	}
	if n, err := f.Write(secret.Data["key.pem"]); n != len(secret.Data["key.pem"]) || err != nil {
		klog.Error("error in writing key.pem")
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		klog.Error("error in closing key.pem file")
		os.Exit(1)
	}
}
