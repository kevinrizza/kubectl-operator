package action

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

/*
func TestRunPackageNotFound(t *testing.T) {
	var cfg Configuration
	objs := []runtime.Object{}
	cl := fake.NewFakeClient(objs...)
	cfg.Client = cl
	cfg.Namespace = "testNamespace"

	lister := NewOperatorListOperands(&cfg)
	lister.PackageName = "testPackage"
	_, err := lister.Run(context.TODO())
	require.Error(t, err)
}
*/

func TestRunNoCRs(t *testing.T) {
	testNamespace := "test-namespace"
	operatorPackageName := "test-package"
	operatorRef := &v1.RichReference{}
	//operatorRef.Kind = "csv"
	//operatorRef.Name = "csv1"
	//operatorRef.Namespace = testNamespace

	csv := &v1alpha1.ClusterServiceVersion{
		/*
			ObjectMeta: metav1.ObjectMeta{
				Name:      operatorRef.Name,
				Namespace: operatorRef.Namespace,
			},
			Spec: v1alpha1.ClusterServiceVersionSpec{
				CustomResourceDefinitions: v1alpha1.CustomResourceDefinitions{
					Owned: []v1alpha1.CRDDescription{
						v1alpha1.CRDDescription{
							Name:    "my-cool-name",
							Version: "v1",
							Kind:    "my-crd",
						},
					},
				},
			},
			Status: v1alpha1.ClusterServiceVersionStatus{
				Phase: v1alpha1.CSVPhaseSucceeded,
			},
		*/
	}

	operator := &v1.Operator{
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorPackageName,
		},
		Status: v1.OperatorStatus{
			Components: &v1.Components{
				Refs: []v1.RichReference{
					*operatorRef,
				},
			},
		},
	}

	cfg := Configuration{}
	cfg.Load()
	objs := []runtime.Object{operator, csv}
	cl := fake.NewFakeClient(objs...)
	cfg.Client = cl
	defaultScheme, err := DefaultScheme()
	require.NoError(t, err)
	cfg.Scheme = defaultScheme
	cfg.Namespace = testNamespace

	key := types.NamespacedName{
		Name: fmt.Sprintf("%s", operatorPackageName),
	}

	foperator := v1.Operator{}
	err = cl.Get(context.TODO(), key, &foperator)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			fmt.Printf("package %s not found", key.Name)
			//return nil, fmt.Errorf("package %s not found", key.Name)
		}
		fmt.Println(err.Error())
	}

	fmt.Println(foperator.Name)

	/*
		lister := NewOperatorListOperands(&cfg)
		lister.PackageName = operatorPackageName

		list, err := lister.Run(context.TODO())
		require.NoError(t, err)
		require.Equal(t, len(list.Items), 0)
	*/
}
