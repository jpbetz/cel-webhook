package informers

import (
	"os"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Registry interface {
	RegisterCustomResourceDefinition(crd *v1.CustomResourceDefinition)
}

func StartCRDInformer(registry Registry, stopCh chan struct{}) error {
	kubeconfig := os.Getenv("KUBECONFIG")

	// Create the client configuration
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}

	// Create the client
	clientset, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return err
	}
	factory := apiextensionsinformers.NewSharedInformerFactory(clientset, time.Minute)
	informer := factory.Apiextensions().V1().CustomResourceDefinitions().Informer()

	// Kubernetes serves an utility to handle API crashes
	defer runtime.HandleCrash()
	// This is the part where your custom code gets triggered based on the
	// event that the shared informer catches
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// When a new pod gets created
		AddFunc:    func(obj interface{}) {
			if crd, ok := obj.(*v1.CustomResourceDefinition); ok {
				registry.RegisterCustomResourceDefinition(crd)
			}
		},
		// When a pod gets updated
		UpdateFunc: func(old interface{}, obj interface{}) {
			if crd, ok := obj.(*v1.CustomResourceDefinition); ok {
				registry.RegisterCustomResourceDefinition(crd)
			}
		},
		// When a pod gets deleted
		DeleteFunc: func(interface{}) {
			// TODO: unregister
		},
	})
	// You need to start the informer, in my case, it runs in the background
	go informer.Run(stopCh)
	return nil
}