# =============================
# ---------- Environment ------
# =============================

ifneq ("$(wildcard ../.env)","")
include ../.env
export
endif

# =============================
# ---------- Project ----------
# =============================

current_dir	=	$(CURDIR)
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
gofmt		=	go fmt
gocover		=	go tool cover
gotest		=	go test
govet		=	go vet
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
	@ echo "Checking the code for common mistakes..."
	@ $(govet) $(modules)
	@ echo "Finished checking."

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

# Admin and maintenance commands
create-admin:
	@ test -n "$(EMAIL)" && test -n "$(PASSWORD)" || { echo "Usage: make create-admin EMAIL=you@example.com PASSWORD=your-password"; exit 1; }
	$(gorun) ./cmd/createadmin -email "$(EMAIL)" -password "$(PASSWORD)"

backfill-exif:
	@ echo "Re-reading EXIF for stored photos (back up the database first)..."
	$(gorun) ./cmd/backfill-exif

# Database Commands
init-db:
	@$(MAKE) -f Makefile.db init
shell-db:
	@$(MAKE) -f Makefile.db shell
backup-db:
	@$(MAKE) -f Makefile.db backup
restore-db:
	@$(MAKE) -f Makefile.db restore BACKUP_FILE="$(BACKUP_FILE)"
clear-db:
	@$(MAKE) -f Makefile.db clear
reset-db:
	@$(MAKE) -f Makefile.db reset

# Prints the manual, a list of available makefile commands that can be used for the server application.
help:
	@ echo "Manual for Server Application"
	@ echo ""
	@ echo "Development:"
	@ echo "  run              runs the server (build first)"
	@ echo "  build            builds the server binary into bin/"
	@ echo "  format           formats the code to the Go standards"
	@ echo "  lint             runs go vet on the code"
	@ echo "  test             runs the tests with coverage for all packages"
	@ echo "  coverage         runs the tests and writes a test coverage report"
	@ echo "  all              cleans, formats, builds and tests"
	@ echo "  clean            deletes the generated files"
	@ echo ""
	@ echo "Admin and maintenance:"
	@ echo "  create-admin     creates an admin user: make create-admin EMAIL=... PASSWORD=..."
	@ echo "  backfill-exif    re-reads EXIF for stored photos (safe to re-run)"
	@ echo ""
	@ echo "Database (see Makefile.db):"
	@ echo "  init-db          creates the database from db/schema.sql (safe to re-run)"
	@ echo "  shell-db         opens the database in the sqlite3 shell"
	@ echo "  backup-db        copies the database into the backup folder"
	@ echo "  restore-db       restores a backup: make restore-db BACKUP_FILE=..."
	@ echo "  reset-db         deletes and recreates the database (destructive)"
	@ echo "  clear-db         same as reset-db"
	@ echo ""
	@ echo "  help             prints this menu again"
