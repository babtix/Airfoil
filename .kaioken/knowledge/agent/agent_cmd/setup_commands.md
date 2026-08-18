go build -ldflags "-X main.version=$(git describe --tags --always)" ./agent/cmd/airfoil  # builds binary with version
./airfoil --help                    # shows commands
./airfoil version                   # prints version
./airfoil doctor                    # validates configuration
./airfoil --config ./config --data ./data --since 48h  # example run with overrides
