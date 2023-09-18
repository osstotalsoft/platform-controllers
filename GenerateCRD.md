## Prerequisites

1. Install go v1.20 from [here](https://golang.org/doc/install)

2. Install make 
```bash
apt install make
```
3. Install the kubernetes code generator (from outside the project folder):
```bash
go install k8s.io/code-generator@v0.28.2
```


## Generate client
Go to repository root and run the following make task:
```bash
make generate-apis
make generate-crd
```