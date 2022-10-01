# =============================
# ---------- Project ----------
# =============================

current_dir	=	${PWD}
bin_dir		=	${current_dir}/bin
out_dir		=	${current_dir}/out
binary		=	server
project		=	server
main		=	main.go
coverage	=	coverage.out


# =================================
# ---------- Go Commands ----------
# =================================

gobuild		=	go build
gorun		=	go run
goget		=	go get
gofmt		=	got fmt
golint		=	golint
gotest		=	go test
gocover		=	go tool cover
ldflags		=	"-s -w"
modules		=	./...


# =======================================
# ---------- Makefile Commands ----------
# =======================================

clean:
	@ echo "Cleaning project repository..."
	rm -rf ${bin_dir}
	rm -rf ${out_dir}
	rm -rf ${coverage}
	@ echo "Finished cleaning project repository."

format:
	@ echo "Formatting code..."
	@ $(gofmt) $(modules)
	@ echo "Finished formatting code."

lint:
	@ echo "Linting the code to Go standards..."
	@ $(golint) $(modules)
	@ echo "Finished linting."

build:
	@ echo "Building server binaries..."
	$(gobuild) $(modules)
	$(gobuild) -o ${bin_dir}/${binary} ./
	@ echo "Finished building server binaries."

run:
	@ echo "Running server binary..."
	${bin_dir}/${binary}

test:
	@ echo "Running tests with package level coverage..."
	$(gotest) $(modules) -v -cover
	@ echo "Finished running tests."

coverage:
	@ echo "Running tests and printing coverage report..."
	@ -$(gotest) $(modules) -race -v -covermode=atomic -coverprofile $(coverage)
	@ $(gocover) -func $(coverage)
	@ echo "Finished writing test coverage report."

# Runs a suite of makefile commands to clean, format, build and test the server application codebase.
all: clean format build test

# Prints the manual, a list of available makefile commands that can be used for the server application.
help:
	@ echo "\
	Manual for Server Application\n\
	\n\
	Available commands:\n\
	--------------------\n\
	* clean		deletes the generated files\n\
	* format	formats the code to the Go standards\n\
	* lint		runs the linter on the code, highlighting code smells\n\
	* build		builds the server binary files\n\
	* run		runs the server\n\
	* test		runs the tests with coverage for all packages in the project\n\
	* coverage	runs the tests and writes a test coverage report\n\
	* all		formats, cleans, builds and tests the binaries\n\
	* help		prints this menu again\n\"
