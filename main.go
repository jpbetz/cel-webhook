package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"

	v1 "k8s.io/api/admission/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"

	"github.com/jpbetz/omni-webhook/informers"
	"github.com/jpbetz/omni-webhook/validators"
)

var (
	certFile string
	keyFile  string
	port     int
)

// CmdWebhook is used by agnhost Cobra.
var CmdWebhook = &cobra.Command{
	Use:   "webhook",
	Short: "Starts a Kubernetes webhook that performs custom validation",
	Long:  `Starts a Kubernetes webhook that performs custom validation`,
	Args:  cobra.MaximumNArgs(0),
	Run:   runCmdWebhook,
}

func init() {
	CmdWebhook.Flags().StringVar(&certFile, "tls-cert-file", "",
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	CmdWebhook.Flags().StringVar(&keyFile, "tls-private-key-file", "",
		"File containing the default x509 private key matching --tls-cert-file.")
	CmdWebhook.Flags().IntVar(&port, "port", 443,
		"Secure port that the webhook listens on")
}

// admitv1beta1Func handles a v1 admission
type admitv1Func func(v1.AdmissionReview) *v1.AdmissionResponse

type convertv1Func func(extensionsv1.ConversionReview) *extensionsv1.ConversionResponse

// serve handles the http portion of a request prior to handing to an admit
// function
func serve(w http.ResponseWriter, r *http.Request, admit admitv1Func, convert convertv1Func) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	//klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	deserializer := codecs.UniversalDeserializer()
	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	switch *gvk {
	case v1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*v1.AdmissionReview)
		if !ok {
			klog.Errorf("Expected v1.AdmissionReview but got: %T", obj)
			return
		}
		responseAdmissionReview := &v1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
	case extensionsv1.SchemeGroupVersion.WithKind("ConversionReview"):
		requestedConversionReview, ok := obj.(*extensionsv1.ConversionReview)
		if !ok {
			klog.Errorf("Expected v1.ConversionReview but got: %T", obj)
			return
		}
		responseConversionReview := &extensionsv1.ConversionReview{}
		responseConversionReview.SetGroupVersionKind(*gvk)
		responseConversionReview.Response = convert(*requestedConversionReview)
		responseConversionReview.Response.UID = requestedConversionReview.Request.UID
		responseObj = responseConversionReview
	default:
		msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

func runCmdWebhook(cmd *cobra.Command, args []string) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	validator := newFormatValidators()
	err := informers.StartCRDInformer(validator, stopCh)
	if err != nil {
		panic(err)
	}

	wasmValidator := validators.NewWasmValidator()
	err = wasmValidator.RegisterModule("example/main.wasm")
	if err != nil {
		panic(err)
	}
	validator.registerFormat("wasm", wasmValidator)

	celValidator := validators.NewCelValidator()
	validator.registerFormat("rule", celValidator)
	validator.registerConverter("conversion", celValidator)

	config := Config{
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	http.HandleFunc("/validate", validator.serveValidateRequest)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("ok")) })
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: configTLS(config),
	}
	err = server.ListenAndServeTLS("", "")
	if err != nil {
		panic(err)
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "runner-webhook",
		Version: "0.0.1",
	}

	rootCmd.AddCommand(CmdWebhook)
	loggingFlags := &flag.FlagSet{}
	klog.InitFlags(loggingFlags)
	rootCmd.PersistentFlags().AddGoFlagSet(loggingFlags)
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
