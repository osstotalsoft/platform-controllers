## Prerequisites

1. Install go v1.20 from [here](https://golang.org/doc/install)

2. Install make 
```bash
apt install make
```
3. Install the kubernetes code generator (from outside the project folder):
```bash
go get k8s.io/code-generator@v0.27.2
sudo chmod 777 ~/go/pkg/mod/k8s.io/code-generator@v0.27.2/generate-groups.sh
```


## Generate client
Go to repository root and run the following make task:
```bash
make generate-apis
make generate-crd
```