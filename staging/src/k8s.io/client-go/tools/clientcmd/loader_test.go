/*
Copyright 2014 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clientcmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
)

var (
	testConfigAlfa = clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"red-user": {Token: "red-token"}},
		Clusters: map[string]*clientcmdapi.Cluster{
			"cow-cluster": {Server: "http://cow.org:8080"}},
		Contexts: map[string]*clientcmdapi.Context{
			"federal-context": {AuthInfo: "red-user", Cluster: "cow-cluster", Namespace: "hammer-ns"}},
	}
	testConfigBravo = clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"black-user": {Token: "black-token"}},
		Clusters: map[string]*clientcmdapi.Cluster{
			"pig-cluster": {Server: "http://pig.org:8080"}},
		Contexts: map[string]*clientcmdapi.Context{
			"queen-anne-context": {AuthInfo: "black-user", Cluster: "pig-cluster", Namespace: "saw-ns"}},
	}
	testConfigCharlie = clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"green-user": {Token: "green-token"}},
		Clusters: map[string]*clientcmdapi.Cluster{
			"horse-cluster": {Server: "http://horse.org:8080"}},
		Contexts: map[string]*clientcmdapi.Context{
			"shaker-context": {AuthInfo: "green-user", Cluster: "horse-cluster", Namespace: "chisel-ns"}},
	}
	testConfigDelta = clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"blue-user": {Token: "blue-token"}},
		Clusters: map[string]*clientcmdapi.Cluster{
			"chicken-cluster": {Server: "http://chicken.org:8080"}},
		Contexts: map[string]*clientcmdapi.Context{
			"gothic-context": {AuthInfo: "blue-user", Cluster: "chicken-cluster", Namespace: "plane-ns"}},
	}

	testConfigConflictAlfa = clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"red-user":    {Token: "a-different-red-token"},
			"yellow-user": {Token: "yellow-token"}},
		Clusters: map[string]*clientcmdapi.Cluster{
			"cow-cluster":    {Server: "http://a-different-cow.org:8080", InsecureSkipTLSVerify: true},
			"donkey-cluster": {Server: "http://donkey.org:8080", InsecureSkipTLSVerify: true}},
		CurrentContext: "federal-context",
	}
)

func TestNonExistentCommandLineFile(t *testing.T) {
	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: "bogus_file",
	}

	_, err := loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for missing command-line file, got none")
	}
	if !strings.Contains(err.Error(), "bogus_file") {
		t.Fatalf("Expected error about 'bogus_file', got %s", err.Error())
	}
}

func TestToleratingMissingFiles(t *testing.T) {
	loadingRules := ClientConfigLoadingRules{
		Precedence: []string{"bogus1", "bogus2", "bogus3"},
	}

	_, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestErrorReadingFile(t *testing.T) {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())

	if err := ioutil.WriteFile(commandLineFile.Name(), []byte("bogus value"), 0644); err != nil {
		t.Fatalf("Error creating tempfile: %v", err)
	}

	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: commandLineFile.Name(),
	}

	_, err := loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for unloadable file, got none")
	}
	if !strings.Contains(err.Error(), commandLineFile.Name()) {
		t.Fatalf("Expected error about '%s', got %s", commandLineFile.Name(), err.Error())
	}
}

func TestErrorReadingNonFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Couldn't create tmpdir")
	}
	defer os.RemoveAll(tmpdir)

	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: tmpdir,
	}

	_, err = loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for non-file, got none")
	}
	if !strings.Contains(err.Error(), tmpdir) {
		t.Fatalf("Expected error about '%s', got %s", tmpdir, err.Error())
	}
}

func TestConflictingCurrentContext(t *testing.T) {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())

	mockCommandLineConfig := clientcmdapi.Config{
		CurrentContext: "any-context-value",
	}
	mockEnvVarConfig := clientcmdapi.Config{
		CurrentContext: "a-different-context",
	}

	WriteToFile(mockCommandLineConfig, commandLineFile.Name())
	WriteToFile(mockEnvVarConfig, envVarFile.Name())

	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: commandLineFile.Name(),
		Precedence:   []string{envVarFile.Name()},
	}

	mergedConfig, err := loadingRules.Load()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if mergedConfig.CurrentContext != mockCommandLineConfig.CurrentContext {
		t.Errorf("expected %v, got %v", mockCommandLineConfig.CurrentContext, mergedConfig.CurrentContext)
	}
}

func TestLoadingEmptyMaps(t *testing.T) {
	configFile, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile.Name())

	mockConfig := clientcmdapi.Config{
		CurrentContext: "any-context-value",
	}

	WriteToFile(mockConfig, configFile.Name())

	config, err := LoadFromFile(configFile.Name())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if config.Clusters == nil {
		t.Error("expected config.Clusters to be non-nil")
	}
	if config.AuthInfos == nil {
		t.Error("expected config.AuthInfos to be non-nil")
	}
	if config.Contexts == nil {
		t.Error("expected config.Contexts to be non-nil")
	}
}

func TestDuplicateClusterName(t *testing.T) {
	configFile, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile.Name())

	err := ioutil.WriteFile(configFile.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster
- cluster:
    api-version: v2
    server: https://test.example.server:443
    certificate-authority: /var/run/secrets/test.example.io/serviceaccount/ca.crt
  name: kubeconfig-cluster
contexts:
- context:
    cluster: kubeconfig-cluster
    namespace: default
    user: kubeconfig-user
  name: kubeconfig-context
current-context: kubeconfig-context
users:
- name: kubeconfig-user
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = LoadFromFile(configFile.Name())
	if err == nil || !strings.Contains(err.Error(),
		"error converting *[]NamedCluster into *map[string]*api.Cluster: duplicate name \"kubeconfig-cluster\" in list") {
		t.Error("Expected error in loading duplicate cluster name, got none")
	}
}

func TestDuplicateContextName(t *testing.T) {
	configFile, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile.Name())

	err := ioutil.WriteFile(configFile.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster
contexts:
- context:
    cluster: kubeconfig-cluster
    namespace: default
    user: kubeconfig-user
  name: kubeconfig-context
- context:
    cluster: test-example-cluster
    namespace: test-example
    user: test-example-user
  name: kubeconfig-context
current-context: kubeconfig-context
users:
- name: kubeconfig-user
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = LoadFromFile(configFile.Name())
	if err == nil || !strings.Contains(err.Error(),
		"error converting *[]NamedContext into *map[string]*api.Context: duplicate name \"kubeconfig-context\" in list") {
		t.Error("Expected error in loading duplicate context name, got none")
	}
}

func TestDuplicateUserName(t *testing.T) {
	configFile, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile.Name())

	err := ioutil.WriteFile(configFile.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster
contexts:
- context:
    cluster: kubeconfig-cluster
    namespace: default
    user: kubeconfig-user
  name: kubeconfig-context
current-context: kubeconfig-context
users:
- name: kubeconfig-user
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
- name: kubeconfig-user
  user:
    tokenFile: /var/run/secrets/test.example.com/serviceaccount/token
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = LoadFromFile(configFile.Name())
	if err == nil || !strings.Contains(err.Error(),
		"error converting *[]NamedAuthInfo into *map[string]*api.AuthInfo: duplicate name \"kubeconfig-user\" in list") {
		t.Error("Expected error in loading duplicate user name, got none")
	}
}

func TestDuplicateExtensionName(t *testing.T) {
	configFile, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile.Name())

	err := ioutil.WriteFile(configFile.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster
contexts:
- context:
    cluster: kubeconfig-cluster
    namespace: default
    user: kubeconfig-user
  name: kubeconfig-context
current-context: kubeconfig-context
users:
- name: kubeconfig-user
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
extensions:
- extension:
    bytes: test
  name: test-extension
- extension:
    bytes: some-example
  name: test-extension
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = LoadFromFile(configFile.Name())
	if err == nil || !strings.Contains(err.Error(),
		"error converting *[]NamedExtension into *map[string]runtime.Object: duplicate name \"test-extension\" in list") {
		t.Error("Expected error in loading duplicate extension name, got none")
	}
}

func TestResolveRelativePaths(t *testing.T) {
	pathResolutionConfig1 := clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"relative-user-1": {ClientCertificate: "relative/client/cert", ClientKey: "../relative/client/key"},
			"absolute-user-1": {ClientCertificate: "/absolute/client/cert", ClientKey: "/absolute/client/key"},
			"relative-cmd-1":  {Exec: &clientcmdapi.ExecConfig{Command: "../relative/client/cmd"}},
			"absolute-cmd-1":  {Exec: &clientcmdapi.ExecConfig{Command: "/absolute/client/cmd"}},
			"PATH-cmd-1":      {Exec: &clientcmdapi.ExecConfig{Command: "cmd"}},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"relative-server-1": {CertificateAuthority: "../relative/ca"},
			"absolute-server-1": {CertificateAuthority: "/absolute/ca"},
		},
	}
	pathResolutionConfig2 := clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"relative-user-2": {ClientCertificate: "relative/client/cert2", ClientKey: "../relative/client/key2"},
			"absolute-user-2": {ClientCertificate: "/absolute/client/cert2", ClientKey: "/absolute/client/key2"},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"relative-server-2": {CertificateAuthority: "../relative/ca2"},
			"absolute-server-2": {CertificateAuthority: "/absolute/ca2"},
		},
	}

	configDir1, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(configDir1)
	configFile1 := path.Join(configDir1, ".kubeconfig")
	configDir1, _ = filepath.Abs(configDir1)

	configDir2, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(configDir2)
	configDir2, _ = ioutil.TempDir(configDir2, "")
	configFile2 := path.Join(configDir2, ".kubeconfig")
	configDir2, _ = filepath.Abs(configDir2)

	WriteToFile(pathResolutionConfig1, configFile1)
	WriteToFile(pathResolutionConfig2, configFile2)

	loadingRules := ClientConfigLoadingRules{
		Precedence: []string{configFile1, configFile2},
	}

	mergedConfig, err := loadingRules.Load()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	foundClusterCount := 0
	for key, cluster := range mergedConfig.Clusters {
		if key == "relative-server-1" {
			foundClusterCount++
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.Clusters["relative-server-1"].CertificateAuthority), cluster.CertificateAuthority, t)
		}
		if key == "relative-server-2" {
			foundClusterCount++
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.Clusters["relative-server-2"].CertificateAuthority), cluster.CertificateAuthority, t)
		}
		if key == "absolute-server-1" {
			foundClusterCount++
			matchStringArg(pathResolutionConfig1.Clusters["absolute-server-1"].CertificateAuthority, cluster.CertificateAuthority, t)
		}
		if key == "absolute-server-2" {
			foundClusterCount++
			matchStringArg(pathResolutionConfig2.Clusters["absolute-server-2"].CertificateAuthority, cluster.CertificateAuthority, t)
		}
	}
	if foundClusterCount != 4 {
		t.Errorf("Expected 4 clusters, found %v: %v", foundClusterCount, mergedConfig.Clusters)
	}

	foundAuthInfoCount := 0
	for key, authInfo := range mergedConfig.AuthInfos {
		if key == "relative-user-1" {
			foundAuthInfoCount++
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.AuthInfos["relative-user-1"].ClientCertificate), authInfo.ClientCertificate, t)
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.AuthInfos["relative-user-1"].ClientKey), authInfo.ClientKey, t)
		}
		if key == "relative-user-2" {
			foundAuthInfoCount++
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.AuthInfos["relative-user-2"].ClientCertificate), authInfo.ClientCertificate, t)
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.AuthInfos["relative-user-2"].ClientKey), authInfo.ClientKey, t)
		}
		if key == "absolute-user-1" {
			foundAuthInfoCount++
			matchStringArg(pathResolutionConfig1.AuthInfos["absolute-user-1"].ClientCertificate, authInfo.ClientCertificate, t)
			matchStringArg(pathResolutionConfig1.AuthInfos["absolute-user-1"].ClientKey, authInfo.ClientKey, t)
		}
		if key == "absolute-user-2" {
			foundAuthInfoCount++
			matchStringArg(pathResolutionConfig2.AuthInfos["absolute-user-2"].ClientCertificate, authInfo.ClientCertificate, t)
			matchStringArg(pathResolutionConfig2.AuthInfos["absolute-user-2"].ClientKey, authInfo.ClientKey, t)
		}
		if key == "relative-cmd-1" {
			foundAuthInfoCount++
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.AuthInfos[key].Exec.Command), authInfo.Exec.Command, t)
		}
		if key == "absolute-cmd-1" {
			foundAuthInfoCount++
			matchStringArg(pathResolutionConfig1.AuthInfos[key].Exec.Command, authInfo.Exec.Command, t)
		}
		if key == "PATH-cmd-1" {
			foundAuthInfoCount++
			matchStringArg(pathResolutionConfig1.AuthInfos[key].Exec.Command, authInfo.Exec.Command, t)
		}
	}
	if foundAuthInfoCount != 7 {
		t.Errorf("Expected 7 users, found %v: %v", foundAuthInfoCount, mergedConfig.AuthInfos)
	}

}

func TestMigratingFile(t *testing.T) {
	sourceFile, _ := ioutil.TempFile("", "")
	defer os.Remove(sourceFile.Name())
	destinationFile, _ := ioutil.TempFile("", "")
	// delete the file so that we'll write to it
	os.Remove(destinationFile.Name())

	WriteToFile(testConfigAlfa, sourceFile.Name())

	loadingRules := ClientConfigLoadingRules{
		MigrationRules: map[string]string{destinationFile.Name(): sourceFile.Name()},
	}

	if _, err := loadingRules.Load(); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	// the load should have recreated this file
	defer os.Remove(destinationFile.Name())

	sourceContent, err := ioutil.ReadFile(sourceFile.Name())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	destinationContent, err := ioutil.ReadFile(destinationFile.Name())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if !reflect.DeepEqual(sourceContent, destinationContent) {
		t.Errorf("source and destination do not match")
	}
}

func TestMigratingFileLeaveExistingFileAlone(t *testing.T) {
	sourceFile, _ := ioutil.TempFile("", "")
	defer os.Remove(sourceFile.Name())
	destinationFile, _ := ioutil.TempFile("", "")
	defer os.Remove(destinationFile.Name())

	WriteToFile(testConfigAlfa, sourceFile.Name())

	loadingRules := ClientConfigLoadingRules{
		MigrationRules: map[string]string{destinationFile.Name(): sourceFile.Name()},
	}

	if _, err := loadingRules.Load(); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	destinationContent, err := ioutil.ReadFile(destinationFile.Name())
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if len(destinationContent) > 0 {
		t.Errorf("destination should not have been touched")
	}
}

func TestMigratingFileSourceMissingSkip(t *testing.T) {
	sourceFilename := "some-missing-file"
	destinationFile, _ := ioutil.TempFile("", "")
	// delete the file so that we'll write to it
	os.Remove(destinationFile.Name())

	loadingRules := ClientConfigLoadingRules{
		MigrationRules: map[string]string{destinationFile.Name(): sourceFilename},
	}

	if _, err := loadingRules.Load(); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if _, err := os.Stat(destinationFile.Name()); !os.IsNotExist(err) {
		t.Errorf("destination should not exist")
	}
}

func TestFileLocking(t *testing.T) {
	f, _ := ioutil.TempFile("", "")
	defer os.Remove(f.Name())

	err := lockFile(f.Name())
	if err != nil {
		t.Errorf("unexpected error while locking file: %v", err)
	}
	defer unlockFile(f.Name())

	err = lockFile(f.Name())
	if err == nil {
		t.Error("expected error while locking file.")
	}
}

func Example_noMergingOnExplicitPaths() {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())

	WriteToFile(testConfigAlfa, commandLineFile.Name())
	WriteToFile(testConfigConflictAlfa, envVarFile.Name())

	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: commandLineFile.Name(),
		Precedence:   []string{envVarFile.Name()},
	}

	mergedConfig, err := loadingRules.Load()

	json, err := runtime.Encode(clientcmdlatest.Codec, mergedConfig)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	output, err := yaml.JSONToYAML(json)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}

	fmt.Printf("%v", string(output))
	// Output:
	// apiVersion: v1
	// clusters:
	// - cluster:
	//     server: http://cow.org:8080
	//   name: cow-cluster
	// contexts:
	// - context:
	//     cluster: cow-cluster
	//     namespace: hammer-ns
	//     user: red-user
	//   name: federal-context
	// current-context: ""
	// kind: Config
	// preferences: {}
	// users:
	// - name: red-user
	//   user:
	//     token: red-token
}

func Example_mergingSomeWithConflict() {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())

	WriteToFile(testConfigAlfa, commandLineFile.Name())
	WriteToFile(testConfigConflictAlfa, envVarFile.Name())

	loadingRules := ClientConfigLoadingRules{
		Precedence: []string{commandLineFile.Name(), envVarFile.Name()},
	}

	mergedConfig, err := loadingRules.Load()

	json, err := runtime.Encode(clientcmdlatest.Codec, mergedConfig)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	output, err := yaml.JSONToYAML(json)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}

	fmt.Printf("%v", string(output))
	// Output:
	// apiVersion: v1
	// clusters:
	// - cluster:
	//     server: http://cow.org:8080
	//   name: cow-cluster
	// - cluster:
	//     insecure-skip-tls-verify: true
	//     server: http://donkey.org:8080
	//   name: donkey-cluster
	// contexts:
	// - context:
	//     cluster: cow-cluster
	//     namespace: hammer-ns
	//     user: red-user
	//   name: federal-context
	// current-context: federal-context
	// kind: Config
	// preferences: {}
	// users:
	// - name: red-user
	//   user:
	//     token: red-token
	// - name: yellow-user
	//   user:
	//     token: yellow-token
}

func Example_mergingEverythingNoConflicts() {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())
	currentDirFile, _ := ioutil.TempFile("", "")
	defer os.Remove(currentDirFile.Name())
	homeDirFile, _ := ioutil.TempFile("", "")
	defer os.Remove(homeDirFile.Name())

	WriteToFile(testConfigAlfa, commandLineFile.Name())
	WriteToFile(testConfigBravo, envVarFile.Name())
	WriteToFile(testConfigCharlie, currentDirFile.Name())
	WriteToFile(testConfigDelta, homeDirFile.Name())

	loadingRules := ClientConfigLoadingRules{
		Precedence: []string{commandLineFile.Name(), envVarFile.Name(), currentDirFile.Name(), homeDirFile.Name()},
	}

	mergedConfig, err := loadingRules.Load()

	json, err := runtime.Encode(clientcmdlatest.Codec, mergedConfig)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	output, err := yaml.JSONToYAML(json)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}

	fmt.Printf("%v", string(output))
	// Output:
	// 	apiVersion: v1
	// clusters:
	// - cluster:
	//     server: http://chicken.org:8080
	//   name: chicken-cluster
	// - cluster:
	//     server: http://cow.org:8080
	//   name: cow-cluster
	// - cluster:
	//     server: http://horse.org:8080
	//   name: horse-cluster
	// - cluster:
	//     server: http://pig.org:8080
	//   name: pig-cluster
	// contexts:
	// - context:
	//     cluster: cow-cluster
	//     namespace: hammer-ns
	//     user: red-user
	//   name: federal-context
	// - context:
	//     cluster: chicken-cluster
	//     namespace: plane-ns
	//     user: blue-user
	//   name: gothic-context
	// - context:
	//     cluster: pig-cluster
	//     namespace: saw-ns
	//     user: black-user
	//   name: queen-anne-context
	// - context:
	//     cluster: horse-cluster
	//     namespace: chisel-ns
	//     user: green-user
	//   name: shaker-context
	// current-context: ""
	// kind: Config
	// preferences: {}
	// users:
	// - name: black-user
	//   user:
	//     token: black-token
	// - name: blue-user
	//   user:
	//     token: blue-token
	// - name: green-user
	//   user:
	//     token: green-token
	// - name: red-user
	//   user:
	//     token: red-token
}

func TestDeduplicate(t *testing.T) {
	testCases := []struct {
		src    []string
		expect []string
	}{
		{
			src:    []string{"a", "b", "c", "d", "e", "f"},
			expect: []string{"a", "b", "c", "d", "e", "f"},
		},
		{
			src:    []string{"a", "b", "c", "b", "e", "f"},
			expect: []string{"a", "b", "c", "e", "f"},
		},
		{
			src:    []string{"a", "a", "b", "b", "c", "b"},
			expect: []string{"a", "b", "c"},
		},
	}

	for _, testCase := range testCases {
		get := deduplicate(testCase.src)
		if !reflect.DeepEqual(get, testCase.expect) {
			t.Errorf("expect: %v, get: %v", testCase.expect, get)
		}
	}
}

func TestMutilpleConfigfiles(t *testing.T) {
	configFile1, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile1.Name())
	configFile2, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile1.Name())

	err := ioutil.WriteFile(configFile1.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster1
contexts:
- context:
    cluster: kubeconfig-cluster1
    namespace: default
    user: kubeconfig-user1
  name: kubeconfig-context1
current-context: kubeconfig-context1
users:
- name: kubeconfig-user1
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
- name: kubeconfig-user2
  user:
    tokenFile: /var/run/secrets/test.example.com/serviceaccount/token
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = ioutil.WriteFile(configFile2.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster2
contexts:
- context:
    cluster: kubeconfig-cluster
    namespace: default
    user: kubeconfig-user3
  name: kubeconfig-context2
current-context: kubeconfig-context
users:
- name: kubeconfig-user3
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
- name: kubeconfig-user4
  user:
    tokenFile: /var/run/secrets/test.example.com/serviceaccount/token
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: configFile1.Name() + " " + configFile2.Name(),
	}

	mergedConfig, err := loadingRules.Load()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(mergedConfig.AuthInfos) != 4 {
		t.Errorf("expected config.AuthInfos has 4 not %v user auth infos", len(mergedConfig.AuthInfos))
	}

	if len(mergedConfig.Clusters) != 2 {
		t.Errorf("expected config.Clusters has 2 not %v clusters", len(mergedConfig.Clusters))
	}

	if len(mergedConfig.Contexts) != 2 {
		t.Errorf("expected config.Contexts has 2 not %v contexts", len(mergedConfig.Contexts))
	}
}

func TestExistingAndNonExistingConfigFile(t *testing.T) {
	configFile, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile.Name())

	err := ioutil.WriteFile(configFile.Name(), []byte(`
kind: Config
apiVersion: v1
clusters:
- cluster:
    api-version: v1
    server: https://kubernetes.default.svc:443
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  name: kubeconfig-cluster1
contexts:
- context:
    cluster: kubeconfig-cluster1
    namespace: default
    user: kubeconfig-user1
  name: kubeconfig-context1
current-context: kubeconfig-context1
users:
- name: kubeconfig-user1
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
- name: kubeconfig-user2
  user:
    tokenFile: /var/run/secrets/test.example.com/serviceaccount/token
`), os.FileMode(0755))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	fmt.Printf("%v", configFile.Name())
	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: configFile.Name() + " nonexisting_file",
	}

	_, err = loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for missing command-line file, got none")
	}
	if !strings.Contains(err.Error(), "nonexisting_file") {
		t.Fatalf("Expected error about 'nonexisting_file', got %s", err.Error())
	}
}

func TestCommandLineMultipleConfigFile(t *testing.T) {
	configFile1, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile1.Name())
	configFile2, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile2.Name())
	configFile3, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile3.Name())
	configFile4, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile4.Name())

	WriteToFile(testConfigAlfa, configFile1.Name())
	WriteToFile(testConfigBravo, configFile2.Name())
	WriteToFile(testConfigCharlie, configFile3.Name())
	WriteToFile(testConfigDelta, configFile4.Name())
	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: configFile1.Name() + " " + configFile2.Name() + " " + configFile3.Name() + " " + configFile4.Name(),
	}

	conf, err := loadingRules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.AuthInfos) != 4 {
		t.Errorf("expected 4 auth infos")
	}

}

func TestCommandLineMultipleConfigFileWithOneMissing(t *testing.T) {
	configFile1, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile1.Name())
	configFile2, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile2.Name())
	configFile3, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile3.Name())
	configFile4, _ := ioutil.TempFile("", "")
	defer os.Remove(configFile4.Name())

	WriteToFile(testConfigAlfa, configFile1.Name())
	WriteToFile(testConfigBravo, configFile2.Name())
	WriteToFile(testConfigCharlie, configFile3.Name())
	WriteToFile(testConfigDelta, configFile4.Name())
	loadingRules := ClientConfigLoadingRules{
		ExplicitPath: configFile1.Name() + " bogus_file " + configFile2.Name() + " " + configFile3.Name() + " " + configFile4.Name(),
	}

	_, err := loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for missing command-line file, got none")
	}
	if !strings.Contains(err.Error(), "bogus_file") {
		t.Fatalf("Expected error about 'bogus_file', got %s", err.Error())
	}
}
