#!/bin/bash

# MantisDB Production Build Script
# This script builds production-ready binaries for all platforms

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="dist"
BINARY_NAME="mantisdb"
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Build configuration
MAX_RETRIES=${MAX_RETRIES:-3}
RETRY_DELAY=${RETRY_DELAY:-5}
PARALLEL_BUILDS=${PARALLEL_BUILDS:-false}
MAX_PARALLEL_JOBS=${MAX_PARALLEL_JOBS:-$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)}
BUILD_QUEUE_SIZE=${BUILD_QUEUE_SIZE:-$MAX_PARALLEL_JOBS}

# Build optimization configuration
BUILD_OPTIMIZATION=${BUILD_OPTIMIZATION:-"size"}  # size|speed|debug
CGO_ENABLED=${CGO_ENABLED:-"auto"}  # auto|true|false
BUILD_CACHE=${BUILD_CACHE:-"true"}  # true|false
BUILD_CACHE_DIR=${BUILD_CACHE_DIR:-".build-cache"}
CUSTOM_BUILD_FLAGS=${CUSTOM_BUILD_FLAGS:-""}
STRIP_SYMBOLS=${STRIP_SYMBOLS:-"true"}
COMPRESS_BINARIES=${COMPRESS_BINARIES:-"false"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Error codes
readonly E_SUCCESS=0
readonly E_DEPENDENCY_MISSING=1
readonly E_FRONTEND_BUILD_FAILED=2
readonly E_BINARY_BUILD_FAILED=3
readonly E_PACKAGE_FAILED=4
readonly E_CHECKSUM_FAILED=5
readonly E_CLEANUP_FAILED=6

# Global variables for tracking
BUILD_ERRORS=()
BUILD_WARNINGS=()
BUILD_START_TIME=$(date +%s)
BUILD_JOBS=()
BUILD_PROGRESS=()
BUILD_RESULTS=()
BUILD_CACHE_HITS=0
BUILD_CACHE_MISSES=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
    BUILD_WARNINGS+=("$1")
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    BUILD_ERRORS+=("$1")
}

# Error handling functions
handle_error() {
    local exit_code=$1
    local error_message=$2
    local context=${3:-"Unknown"}
    
    log_error "Build failed in $context: $error_message (exit code: $exit_code)"
    
    # Add context-specific troubleshooting
    case $exit_code in
        $E_DEPENDENCY_MISSING)
            log_error "Missing dependencies. Please install required tools."
            ;;
        $E_FRONTEND_BUILD_FAILED)
            log_error "Frontend build failed. Check Node.js version and dependencies."
            ;;
        $E_BINARY_BUILD_FAILED)
            log_error "Binary build failed. Check Go version and source code."
            ;;
        $E_PACKAGE_FAILED)
            log_error "Package creation failed. Check disk space and permissions."
            ;;
    esac
    
    cleanup_on_error
    exit $exit_code
}

cleanup_on_error() {
    log_warning "Cleaning up partial build artifacts..."
    # Keep logs but remove incomplete builds
    if [ -d "$BUILD_DIR" ]; then
        find "$BUILD_DIR" -name "*.tmp" -delete 2>/dev/null || true
        find "$BUILD_DIR" -name "*.partial" -delete 2>/dev/null || true
    fi
}

# Retry mechanism
retry_command() {
    local max_attempts=$1
    local delay=$2
    local command_name=$3
    shift 3
    local command=("$@")
    
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        log_info "Attempting $command_name (attempt $attempt/$max_attempts)"
        
        if "${command[@]}"; then
            log_success "$command_name completed successfully"
            return 0
        else
            local exit_code=$?
            if [ $attempt -eq $max_attempts ]; then
                log_error "$command_name failed after $max_attempts attempts"
                return $exit_code
            else
                log_warning "$command_name failed (attempt $attempt/$max_attempts), retrying in ${delay}s..."
                sleep $delay
            fi
        fi
        ((attempt++))
    done
}

# Dependency checking
check_dependencies() {
    log_info "Checking build dependencies..."
    
    local missing_deps=()
    
    if ! command -v go &> /dev/null; then
        missing_deps+=("go")
    fi
    
    if ! command -v node &> /dev/null; then
        missing_deps+=("node")
    fi
    
    if ! command -v npm &> /dev/null; then
        missing_deps+=("npm")
    fi
    
    if ! command -v git &> /dev/null; then
        missing_deps+=("git")
    fi
    
    # Check for archiving tools
    if ! command -v tar &> /dev/null && ! command -v zip &> /dev/null; then
        missing_deps+=("tar or zip")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_error "Please install the missing tools and try again"
        exit $E_DEPENDENCY_MISSING
    fi
    
    # Check versions
    local go_version=$(go version | cut -d' ' -f3 | sed 's/go//')
    local node_version=$(node --version | sed 's/v//')
    
    log_info "Go version: $go_version"
    log_info "Node.js version: $node_version"
    
    log_success "All dependencies are available"
}

# Build optimization functions
get_optimization_flags() {
    local optimization=$1
    local flags=""
    
    case $optimization in
        "size")
            flags="-s -w -trimpath"
            ;;
        "speed")
            flags="-trimpath"
            ;;
        "debug")
            flags="-N -l"
            ;;
        *)
            log_warning "Unknown optimization level: $optimization, using 'size'"
            flags="-s -w -trimpath"
            ;;
    esac
    
    echo "$flags"
}

determine_cgo_setting() {
    local platform=$1
    local cgo_setting="$CGO_ENABLED"
    
    if [ "$cgo_setting" = "auto" ]; then
        # Auto-detect CGO based on platform and available tools
        local platform_split=(${platform//\// })
        local goos=${platform_split[0]}
        local goarch=${platform_split[1]}
        
        # Check if we have CGO dependencies and cross-compilation support
        if [ "$goos" = "$(go env GOOS)" ] && [ "$goarch" = "$(go env GOARCH)" ]; then
            # Native build - check if CGO is available and beneficial
            if command -v gcc &> /dev/null && [ -f "cgo/storage_engine.c" ]; then
                cgo_setting="1"
                log_info "Auto-enabled CGO for native build ($goos/$goarch)"
            else
                cgo_setting="0"
                log_info "Auto-disabled CGO for native build ($goos/$goarch) - no CGO dependencies or compiler"
            fi
        else
            # Cross-compilation - disable CGO by default for simplicity
            cgo_setting="0"
            log_info "Auto-disabled CGO for cross-compilation ($goos/$goarch)"
        fi
    elif [ "$cgo_setting" = "true" ]; then
        cgo_setting="1"
    elif [ "$cgo_setting" = "false" ]; then
        cgo_setting="0"
    fi
    
    echo "$cgo_setting"
}

setup_build_cache() {
    if [ "$BUILD_CACHE" = "true" ]; then
        log_info "Setting up build cache..."
        
        if ! mkdir -p "$BUILD_CACHE_DIR"; then
            log_warning "Failed to create build cache directory, disabling cache"
            BUILD_CACHE="false"
            return
        fi
        
        # Set Go build cache
        export GOCACHE="$PWD/$BUILD_CACHE_DIR/go-cache"
        export GOMODCACHE="$PWD/$BUILD_CACHE_DIR/mod-cache"
        
        # Create cache directories
        mkdir -p "$GOCACHE" "$GOMODCACHE"
        
        log_success "Build cache enabled at $BUILD_CACHE_DIR"
    else
        log_info "Build cache disabled"
    fi
}

check_build_cache() {
    local platform=$1
    local cache_key="${platform}_${VERSION}_${BUILD_OPTIMIZATION}_$(echo "$CUSTOM_BUILD_FLAGS" | md5sum | cut -d' ' -f1 2>/dev/null || echo 'nocache')"
    local cache_file="$BUILD_CACHE_DIR/builds/$cache_key"
    
    if [ "$BUILD_CACHE" = "true" ] && [ -f "$cache_file" ]; then
        local platform_split=(${platform//\// })
        local goos=${platform_split[0]}
        local goarch=${platform_split[1]}
        local output_name="${BINARY_NAME}-${goos}-${goarch}"
        if [ "$goos" = "windows" ]; then
            output_name+='.exe'
        fi
        
        log_info "Cache hit for $platform, copying cached binary"
        if cp "$cache_file" "$BUILD_DIR/$output_name"; then
            ((BUILD_CACHE_HITS++))
            return 0
        else
            log_warning "Failed to copy cached binary for $platform"
        fi
    fi
    
    ((BUILD_CACHE_MISSES++))
    return 1
}

cache_build_result() {
    local platform=$1
    local binary_path=$2
    
    if [ "$BUILD_CACHE" = "true" ] && [ -f "$binary_path" ]; then
        local cache_key="${platform}_${VERSION}_${BUILD_OPTIMIZATION}_$(echo "$CUSTOM_BUILD_FLAGS" | md5sum | cut -d' ' -f1 2>/dev/null || echo 'nocache')"
        local cache_file="$BUILD_CACHE_DIR/builds/$cache_key"
        
        mkdir -p "$(dirname "$cache_file")"
        if cp "$binary_path" "$cache_file"; then
            log_info "Cached build result for $platform"
        else
            log_warning "Failed to cache build result for $platform"
        fi
    fi
}

compress_binary() {
    local binary_path=$1
    
    if [ "$COMPRESS_BINARIES" = "true" ]; then
        log_info "Compressing binary: $(basename "$binary_path")"
        
        # Try UPX compression if available
        if command -v upx &> /dev/null; then
            if upx --best --lzma "$binary_path" 2>/dev/null; then
                log_success "Binary compressed with UPX"
                return 0
            else
                log_warning "UPX compression failed, keeping original binary"
            fi
        else
            log_warning "UPX not available, skipping binary compression"
        fi
    fi
    
    return 0
}

# Environment setup
setup_build_environment() {
    log_info "Setting up build environment..."
    
    # Clean previous builds
    log_info "Cleaning previous builds..."
    if ! rm -rf "$BUILD_DIR"; then
        handle_error $E_CLEANUP_FAILED "Failed to clean build directory" "cleanup"
    fi
    
    if ! mkdir -p "$BUILD_DIR"; then
        handle_error $E_CLEANUP_FAILED "Failed to create build directory" "setup"
    fi
    
    # Set up build cache
    setup_build_cache
    
    # Set Go environment
    export GOPROXY=${GOPROXY:-"https://proxy.golang.org,direct"}
    export GOSUMDB=${GOSUMDB:-"sum.golang.org"}
    export GOFLAGS=${GOFLAGS:-"-buildvcs=false"}
    
    # Download dependencies once
    log_info "Downloading Go dependencies..."
    if ! go mod download; then
        log_warning "Failed to download some dependencies, continuing anyway"
    fi
    
    log_success "Build environment ready"
}

# Frontend build function
build_frontend() {
    log_info "Building admin dashboard frontend..."
    
    local frontend_dir="admin/frontend"
    
    if [ ! -d "$frontend_dir" ]; then
        log_warning "Frontend directory not found, skipping frontend build"
        return 0
    fi
    
    cd "$frontend_dir" || handle_error $E_FRONTEND_BUILD_FAILED "Cannot access frontend directory" "frontend"
    
    # Install dependencies if needed
    if [ ! -d "node_modules" ]; then
        log_info "Installing frontend dependencies..."
        if ! retry_command $MAX_RETRIES $RETRY_DELAY "npm install" npm install; then
            cd ../..
            handle_error $E_FRONTEND_BUILD_FAILED "Failed to install frontend dependencies" "frontend"
        fi
    fi
    
    # Build frontend
    log_info "Compiling React application..."
    if ! retry_command $MAX_RETRIES $RETRY_DELAY "npm build" npm run build; then
        cd ../..
        handle_error $E_FRONTEND_BUILD_FAILED "Failed to build frontend" "frontend"
    fi
    
    cd ../..
    log_success "Frontend build completed"
}

# Platform-specific build functions
build_platform_binary() {
    local platform=$1
    local platform_split=(${platform//\// })
    local goos=${platform_split[0]}
    local goarch=${platform_split[1]}
    
    local output_name="${BINARY_NAME}-${goos}-${goarch}"
    if [ "$goos" = "windows" ]; then
        output_name+='.exe'
    fi
    
    local binary_path="$BUILD_DIR/$output_name"
    
    # Check build cache first
    if check_build_cache "$platform"; then
        log_success "Used cached binary: $output_name"
        return 0
    fi
    
    log_info "Building binary for $goos/$goarch (optimization: $BUILD_OPTIMIZATION)..."
    
    # Determine CGO setting for this platform
    local cgo_enabled=$(determine_cgo_setting "$platform")
    
    # Get optimization flags
    local opt_flags=$(get_optimization_flags "$BUILD_OPTIMIZATION")
    
    # Build version info
    local build_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    local git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')
    
    # Construct ldflags
    local ldflags="$opt_flags -X main.Version=$VERSION -X main.BuildTime=$build_time -X main.GitCommit=$git_commit"
    
    # Add custom build flags if specified
    local build_flags=""
    if [ -n "$CUSTOM_BUILD_FLAGS" ]; then
        build_flags="$CUSTOM_BUILD_FLAGS"
        log_info "Using custom build flags: $CUSTOM_BUILD_FLAGS"
    fi
    
    # Set build tags based on CGO and optimization
    local build_tags=""
    if [ "$cgo_enabled" = "1" ]; then
        build_tags="cgo"
        log_info "Building with CGO enabled for $goos/$goarch"
    else
        build_tags="purego"
        log_info "Building with pure Go for $goos/$goarch"
    fi
    
    # Perform the build
    local build_start=$(date +%s)
    if ! env GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="$cgo_enabled" go build \
        -ldflags="$ldflags" \
        -tags="$build_tags" \
        $build_flags \
        -o "$binary_path" \
        cmd/mantisDB/main.go; then
        handle_error $E_BINARY_BUILD_FAILED "Failed to build binary for $goos/$goarch" "binary-build"
    fi
    local build_end=$(date +%s)
    local build_duration=$((build_end - build_start))
    
    # Verify binary was created
    if [ ! -f "$binary_path" ]; then
        handle_error $E_BINARY_BUILD_FAILED "Binary file not created for $goos/$goarch" "binary-build"
    fi
    
    # Get binary size
    local binary_size=$(stat -f%z "$binary_path" 2>/dev/null || stat -c%s "$binary_path" 2>/dev/null || echo "unknown")
    
    # Compress binary if requested
    compress_binary "$binary_path"
    
    # Cache the build result
    cache_build_result "$platform" "$binary_path"
    
    log_success "Built binary: $output_name (${binary_size} bytes, ${build_duration}s)"
    return 0
}

# Package creation functions
create_platform_package() {
    local platform=$1
    local platform_split=(${platform//\// })
    local goos=${platform_split[0]}
    local goarch=${platform_split[1]}
    
    local output_name="${BINARY_NAME}-${goos}-${goarch}"
    if [ "$goos" = "windows" ]; then
        output_name+='.exe'
    fi
    
    local package_dir="$BUILD_DIR/${BINARY_NAME}-${goos}-${goarch}"
    
    log_info "Creating package for $goos/$goarch..."
    
    # Create package directory
    if ! mkdir -p "$package_dir"; then
        handle_error $E_PACKAGE_FAILED "Failed to create package directory for $goos/$goarch" "packaging"
    fi
    
    # Copy binary
    if ! cp "$BUILD_DIR/$output_name" "$package_dir/"; then
        handle_error $E_PACKAGE_FAILED "Failed to copy binary for $goos/$goarch" "packaging"
    fi
    
    # Copy documentation
    if ! cp README.md "$package_dir/" 2>/dev/null; then
        log_warning "README.md not found, creating placeholder"
        echo "# MantisDB $VERSION" > "$package_dir/README.md"
    fi
    
    if ! cp LICENSE "$package_dir/" 2>/dev/null; then
        log_warning "LICENSE file not found, creating placeholder"
        echo "# License file not found" > "$package_dir/LICENSE"
    fi
    
    # Create platform-specific installer
    case $goos in
        "linux"|"darwin")
            create_unix_installer "$package_dir" "$goos" "$goarch"
            ;;
        "windows")
            create_windows_installer "$package_dir" "$goarch"
            ;;
    esac
    
    # Create archive
    cd "$BUILD_DIR" || handle_error $E_PACKAGE_FAILED "Cannot access build directory" "packaging"
    
    if [ "$goos" = "windows" ]; then
        if ! zip -r "${BINARY_NAME}-${goos}-${goarch}.zip" "${BINARY_NAME}-${goos}-${goarch}/"; then
            cd ..
            handle_error $E_PACKAGE_FAILED "Failed to create ZIP archive for $goos/$goarch" "packaging"
        fi
    else
        if ! tar -czf "${BINARY_NAME}-${goos}-${goarch}.tar.gz" "${BINARY_NAME}-${goos}-${goarch}/"; then
            cd ..
            handle_error $E_PACKAGE_FAILED "Failed to create TAR archive for $goos/$goarch" "packaging"
        fi
    fi
    
    cd ..
    log_success "Package created for $goos/$goarch"
}

# Progress tracking functions
init_progress_tracking() {
    for platform in "${PLATFORMS[@]}"; do
        BUILD_PROGRESS+=("$platform:pending")
        BUILD_RESULTS+=("$platform:unknown")
    done
}

update_progress() {
    local platform=$1
    local status=$2
    
    for i in "${!BUILD_PROGRESS[@]}"; do
        if [[ "${BUILD_PROGRESS[$i]}" == "$platform:"* ]]; then
            BUILD_PROGRESS[$i]="$platform:$status"
            break
        fi
    done
}

update_result() {
    local platform=$1
    local result=$2
    
    for i in "${!BUILD_RESULTS[@]}"; do
        if [[ "${BUILD_RESULTS[$i]}" == "$platform:"* ]]; then
            BUILD_RESULTS[$i]="$platform:$result"
            break
        fi
    done
}

show_progress() {
    local completed=0
    local total=${#PLATFORMS[@]}
    
    echo -ne "\r${BLUE}Progress:${NC} "
    for progress in "${BUILD_PROGRESS[@]}"; do
        local platform=$(echo "$progress" | cut -d: -f1)
        local status=$(echo "$progress" | cut -d: -f2)
        
        case $status in
            "pending") echo -ne "${YELLOW}⏳${NC}" ;;
            "building") echo -ne "${BLUE}🔨${NC}" ;;
            "packaging") echo -ne "${YELLOW}📦${NC}" ;;
            "completed") echo -ne "${GREEN}✅${NC}"; ((completed++)) ;;
            "failed") echo -ne "${RED}❌${NC}"; ((completed++)) ;;
        esac
    done
    echo -ne " ($completed/$total)"
}

# Parallel build functions
build_platform_parallel() {
    local platform=$1
    local job_id=$2
    local temp_log="/tmp/mantisdb_build_${job_id}.log"
    
    {
        update_progress "$platform" "building"
        
        if build_platform_binary "$platform" 2>&1; then
            update_progress "$platform" "packaging"
            if create_platform_package "$platform" 2>&1; then
                update_progress "$platform" "completed"
                update_result "$platform" "success"
                echo "SUCCESS:$platform" >> "/tmp/mantisdb_build_results_${job_id}.txt"
            else
                update_progress "$platform" "failed"
                update_result "$platform" "packaging_failed"
                echo "PACKAGING_FAILED:$platform" >> "/tmp/mantisdb_build_results_${job_id}.txt"
            fi
        else
            update_progress "$platform" "failed"
            update_result "$platform" "build_failed"
            echo "BUILD_FAILED:$platform" >> "/tmp/mantisdb_build_results_${job_id}.txt"
        fi
    } > "$temp_log" 2>&1
    
    # Signal completion
    echo "COMPLETED:$platform:$job_id" >> "/tmp/mantisdb_build_completion.txt"
}

# Build queue management
manage_build_queue() {
    local platforms=("$@")
    local active_jobs=0
    local completed_jobs=0
    local total_jobs=${#platforms[@]}
    local job_counter=0
    
    # Initialize tracking files
    rm -f /tmp/mantisdb_build_completion.txt
    rm -f /tmp/mantisdb_build_results_*.txt
    
    log_info "Starting parallel builds with max $MAX_PARALLEL_JOBS concurrent jobs"
    
    # Start initial batch of jobs
    for platform in "${platforms[@]}"; do
        if [ $active_jobs -lt $MAX_PARALLEL_JOBS ]; then
            ((job_counter++))
            build_platform_parallel "$platform" "$job_counter" &
            BUILD_JOBS+=($!)
            ((active_jobs++))
            log_info "Started build job for $platform (job $job_counter)"
        else
            break
        fi
    done
    
    # Process remaining platforms as jobs complete
    local platform_index=$active_jobs
    
    while [ $completed_jobs -lt $total_jobs ]; do
        # Check for completed jobs
        if [ -f "/tmp/mantisdb_build_completion.txt" ]; then
            while IFS= read -r completion_line; do
                if [[ $completion_line == COMPLETED:* ]]; then
                    local completed_platform=$(echo "$completion_line" | cut -d: -f2)
                    local completed_job_id=$(echo "$completion_line" | cut -d: -f3)
                    
                    ((completed_jobs++))
                    ((active_jobs--))
                    
                    log_info "Completed build for $completed_platform"
                    
                    # Start next job if available
                    if [ $platform_index -lt $total_jobs ] && [ $active_jobs -lt $MAX_PARALLEL_JOBS ]; then
                        local next_platform="${platforms[$platform_index]}"
                        ((job_counter++))
                        build_platform_parallel "$next_platform" "$job_counter" &
                        BUILD_JOBS+=($!)
                        ((active_jobs++))
                        ((platform_index++))
                        log_info "Started build job for $next_platform (job $job_counter)"
                    fi
                fi
            done < "/tmp/mantisdb_build_completion.txt"
            
            # Clear processed completions
            > "/tmp/mantisdb_build_completion.txt"
        fi
        
        show_progress
        sleep 1
    done
    
    # Wait for all background jobs to complete
    for job in "${BUILD_JOBS[@]}"; do
        wait "$job" 2>/dev/null || true
    done
    
    echo "" # New line after progress display
    log_info "All parallel builds completed"
}

# Collect parallel build results
collect_build_results() {
    local failed_platforms=()
    
    # Collect results from all job result files
    for result_file in /tmp/mantisdb_build_results_*.txt; do
        if [ -f "$result_file" ]; then
            while IFS= read -r result_line; do
                local status=$(echo "$result_line" | cut -d: -f1)
                local platform=$(echo "$result_line" | cut -d: -f2)
                
                case $status in
                    "SUCCESS")
                        log_success "✓ Completed $platform"
                        ;;
                    "BUILD_FAILED")
                        failed_platforms+=("$platform (build)")
                        ;;
                    "PACKAGING_FAILED")
                        failed_platforms+=("$platform (packaging)")
                        ;;
                esac
            done < "$result_file"
            rm -f "$result_file"
        fi
    done
    
    # Clean up temporary files
    rm -f /tmp/mantisdb_build_completion.txt
    rm -f /tmp/mantisdb_build_*.log
    
    if [ ${#failed_platforms[@]} -ne 0 ]; then
        log_error "Failed platforms: ${failed_platforms[*]}"
        handle_error $E_BINARY_BUILD_FAILED "Some platform builds failed" "parallel-build"
    fi
}

# Build all platforms (with parallel support)
build_all_platforms() {
    log_info "Building binaries for all platforms..."
    
    init_progress_tracking
    
    if [ "$PARALLEL_BUILDS" = "true" ]; then
        log_info "Using parallel builds (max concurrent: $MAX_PARALLEL_JOBS)"
        manage_build_queue "${PLATFORMS[@]}"
        collect_build_results
    else
        log_info "Using sequential builds"
        local failed_platforms=()
        
        for platform in "${PLATFORMS[@]}"; do
            update_progress "$platform" "building"
            if build_platform_binary "$platform"; then
                update_progress "$platform" "packaging"
                if create_platform_package "$platform"; then
                    update_progress "$platform" "completed"
                    log_success "✓ Completed $platform"
                else
                    update_progress "$platform" "failed"
                    failed_platforms+=("$platform (packaging)")
                fi
            else
                update_progress "$platform" "failed"
                failed_platforms+=("$platform (build)")
            fi
            show_progress
        done
        
        echo "" # New line after progress display
        
        if [ ${#failed_platforms[@]} -ne 0 ]; then
            log_error "Failed platforms: ${failed_platforms[*]}"
            handle_error $E_BINARY_BUILD_FAILED "Some platform builds failed" "sequential-build"
        fi
    fi
    
    log_success "All platform builds completed"
}

# Checksum generation
generate_checksums() {
    log_info "Generating checksums..."
    
    cd "$BUILD_DIR" || handle_error $E_CHECKSUM_FAILED "Cannot access build directory" "checksum"
    
    # Generate checksums for archives
    if command -v sha256sum &> /dev/null; then
        if ! sha256sum *.tar.gz *.zip > checksums.txt 2>/dev/null; then
            cd ..
            handle_error $E_CHECKSUM_FAILED "Failed to generate checksums with sha256sum" "checksum"
        fi
    elif command -v shasum &> /dev/null; then
        if ! shasum -a 256 *.tar.gz *.zip > checksums.txt 2>/dev/null; then
            cd ..
            handle_error $E_CHECKSUM_FAILED "Failed to generate checksums with shasum" "checksum"
        fi
    else
        cd ..
        handle_error $E_CHECKSUM_FAILED "No checksum tool available (sha256sum or shasum)" "checksum"
    fi
    
    cd ..
    log_success "Checksums generated"
}

# Build summary
print_build_summary() {
    local build_end_time=$(date +%s)
    local build_duration=$((build_end_time - BUILD_START_TIME))
    
    echo ""
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}Build Summary${NC}"
    echo -e "${BLUE}================================${NC}"
    echo "Version: $VERSION"
    echo "Build Directory: $BUILD_DIR"
    echo "Build Duration: ${build_duration}s"
    echo "Platforms Built: ${#PLATFORMS[@]}"
    echo "Optimization: $BUILD_OPTIMIZATION"
    
    if [ "$BUILD_CACHE" = "true" ]; then
        echo "Cache Hits: $BUILD_CACHE_HITS"
        echo "Cache Misses: $BUILD_CACHE_MISSES"
        local cache_hit_rate=0
        if [ $((BUILD_CACHE_HITS + BUILD_CACHE_MISSES)) -gt 0 ]; then
            cache_hit_rate=$((BUILD_CACHE_HITS * 100 / (BUILD_CACHE_HITS + BUILD_CACHE_MISSES)))
        fi
        echo "Cache Hit Rate: ${cache_hit_rate}%"
    fi
    
    if [ ${#BUILD_WARNINGS[@]} -gt 0 ]; then
        echo -e "${YELLOW}Warnings: ${#BUILD_WARNINGS[@]}${NC}"
        for warning in "${BUILD_WARNINGS[@]}"; do
            echo -e "  ${YELLOW}⚠${NC} $warning"
        done
    fi
    
    if [ ${#BUILD_ERRORS[@]} -gt 0 ]; then
        echo -e "${RED}Errors: ${#BUILD_ERRORS[@]}${NC}"
        for error in "${BUILD_ERRORS[@]}"; do
            echo -e "  ${RED}✗${NC} $error"
        done
    else
        echo -e "${GREEN}Status: Success${NC}"
    fi
    
    echo ""
    echo "Artifacts available in: $BUILD_DIR/"
    echo ""
    echo "Files created:"
    ls -la "$BUILD_DIR/" 2>/dev/null || log_warning "Cannot list build directory contents"
}

# Command line argument parsing
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                VERSION="$2"
                shift 2
                ;;
            --optimization)
                BUILD_OPTIMIZATION="$2"
                shift 2
                ;;
            --cgo)
                CGO_ENABLED="$2"
                shift 2
                ;;
            --parallel)
                PARALLEL_BUILDS="true"
                shift
                ;;
            --no-parallel)
                PARALLEL_BUILDS="false"
                shift
                ;;
            --max-jobs)
                MAX_PARALLEL_JOBS="$2"
                shift 2
                ;;
            --cache)
                BUILD_CACHE="true"
                shift
                ;;
            --no-cache)
                BUILD_CACHE="false"
                shift
                ;;
            --cache-dir)
                BUILD_CACHE_DIR="$2"
                shift 2
                ;;
            --compress)
                COMPRESS_BINARIES="true"
                shift
                ;;
            --no-compress)
                COMPRESS_BINARIES="false"
                shift
                ;;
            --build-flags)
                CUSTOM_BUILD_FLAGS="$2"
                shift 2
                ;;
            --platforms)
                IFS=',' read -ra PLATFORMS <<< "$2"
                shift 2
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << EOF
MantisDB Production Build Script

Usage: $0 [OPTIONS]

Options:
  --version VERSION          Set build version (default: $VERSION)
  --optimization LEVEL       Set optimization level: size|speed|debug (default: $BUILD_OPTIMIZATION)
  --cgo SETTING             CGO setting: auto|true|false (default: $CGO_ENABLED)
  --parallel                Enable parallel builds
  --no-parallel             Disable parallel builds
  --max-jobs N              Maximum parallel jobs (default: auto-detect)
  --cache                   Enable build cache
  --no-cache                Disable build cache
  --cache-dir DIR           Build cache directory (default: $BUILD_CACHE_DIR)
  --compress                Enable binary compression with UPX
  --no-compress             Disable binary compression
  --build-flags FLAGS       Custom build flags
  --platforms LIST          Comma-separated list of platforms (default: all)
  --help, -h                Show this help message

Examples:
  $0                                    # Default build
  $0 --parallel --optimization speed   # Fast parallel build
  $0 --cgo true --platforms linux/amd64,darwin/amd64  # CGO build for specific platforms
  $0 --cache --compress                 # Cached build with compression

Supported platforms: ${PLATFORMS[*]}
EOF
}

# Main execution function
main() {
    # Parse command line arguments
    parse_arguments "$@"
    
    echo -e "${BLUE}MantisDB Production Build${NC}"
    echo -e "${BLUE}========================${NC}"
    echo "Version: $VERSION"
    echo "Build Directory: $BUILD_DIR"
    echo "Max Retries: $MAX_RETRIES"
    echo "Parallel Builds: $PARALLEL_BUILDS"
    if [ "$PARALLEL_BUILDS" = "true" ]; then
        echo "Max Parallel Jobs: $MAX_PARALLEL_JOBS"
        echo "Build Queue Size: $BUILD_QUEUE_SIZE"
    fi
    echo "Target Platforms: ${#PLATFORMS[@]}"
    echo "Build Optimization: $BUILD_OPTIMIZATION"
    echo "CGO Setting: $CGO_ENABLED"
    echo "Build Cache: $BUILD_CACHE"
    if [ "$BUILD_CACHE" = "true" ]; then
        echo "Cache Directory: $BUILD_CACHE_DIR"
    fi
    echo "Strip Symbols: $STRIP_SYMBOLS"
    echo "Compress Binaries: $COMPRESS_BINARIES"
    if [ -n "$CUSTOM_BUILD_FLAGS" ]; then
        echo "Custom Build Flags: $CUSTOM_BUILD_FLAGS"
    fi
    echo ""
    
    # Execute build phases
    check_dependencies
    setup_build_environment
    build_frontend
    build_all_platforms
    generate_checksums
    
    print_build_summary
    
    if [ ${#BUILD_ERRORS[@]} -eq 0 ]; then
        log_success "Build completed successfully!"
        exit $E_SUCCESS
    else
        log_error "Build completed with errors"
        exit $E_BINARY_BUILD_FAILED
    fi
}

# Execute main function if script is run directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi

# Installer creation functions
create_unix_installer() {
    local package_dir=$1
    local os=$2
    local arch=$3
    
    log_info "Creating Unix installer for $os/$arch..."
    
    cat > "$package_dir/install.sh" << 'EOF'
#!/bin/bash

# MantisDB Installer Script

set -e

INSTALL_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"
DATA_DIR="/var/lib/mantisdb"
CONFIG_DIR="/etc/mantisdb"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}MantisDB Installer${NC}"
echo "=================="

# Check if running as root for system installation
if [ "$EUID" -eq 0 ]; then
    echo "Installing system-wide..."
    INSTALL_MODE="system"
else
    echo "Installing for current user..."
    INSTALL_DIR="$HOME/.local/bin"
    SERVICE_DIR="$HOME/.config/systemd/user"
    DATA_DIR="$HOME/.local/share/mantisdb"
    CONFIG_DIR="$HOME/.config/mantisdb"
    INSTALL_MODE="user"
fi

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$CONFIG_DIR"

# Copy binary
echo "Installing MantisDB binary..."
cp mantisdb* "$INSTALL_DIR/mantisdb"
chmod +x "$INSTALL_DIR/mantisdb"

# Create default configuration
echo "Creating default configuration..."
cat > "$CONFIG_DIR/config.yaml" << 'CONFIGEOF'
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "/var/lib/mantisdb"
  cache_size: "256MB"
  buffer_size: "64MB"
  use_cgo: false
  sync_writes: true

security:
  admin_token: ""
  enable_cors: false
  cors_origins: ["http://localhost:3000"]

logging:
  level: "info"
  format: "json"
  output: "stdout"
CONFIGEOF

# Update data directory in config for user installation
if [ "$INSTALL_MODE" = "user" ]; then
    sed -i.bak "s|/var/lib/mantisdb|$DATA_DIR|g" "$CONFIG_DIR/config.yaml"
    rm "$CONFIG_DIR/config.yaml.bak"
fi

# Create systemd service (if systemd is available)
if command -v systemctl >/dev/null 2>&1; then
    echo "Creating systemd service..."
    mkdir -p "$SERVICE_DIR"
    
    cat > "$SERVICE_DIR/mantisdb.service" << SERVICEEOF
[Unit]
Description=MantisDB - Multi-Model Database
After=network.target

[Service]
Type=simple
User=$(whoami)
ExecStart=$INSTALL_DIR/mantisdb --config=$CONFIG_DIR/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SERVICEEOF

    if [ "$INSTALL_MODE" = "system" ]; then
        systemctl daemon-reload
        echo "Service created. Enable with: sudo systemctl enable mantisdb"
        echo "Start with: sudo systemctl start mantisdb"
    else
        systemctl --user daemon-reload
        echo "Service created. Enable with: systemctl --user enable mantisdb"
        echo "Start with: systemctl --user start mantisdb"
    fi
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Configuration file: $CONFIG_DIR/config.yaml"
echo "Data directory: $DATA_DIR"
echo ""
echo "To start MantisDB manually:"
echo "  $INSTALL_DIR/mantisdb --config=$CONFIG_DIR/config.yaml"
echo ""
echo "Admin dashboard will be available at: http://localhost:8081"
EOF

    if ! chmod +x "$package_dir/install.sh"; then
        log_warning "Failed to make installer executable for $os/$arch"
        return 1
    fi
    
    log_success "Unix installer created for $os/$arch"
    return 0
}

create_windows_installer() {
    local package_dir=$1
    local arch=$2
    
    log_info "Creating Windows installer for $arch..."
    
    cat > "$package_dir/install.bat" << 'EOF'
@echo off
echo MantisDB Windows Installer
echo =========================

set INSTALL_DIR=%PROGRAMFILES%\MantisDB
set DATA_DIR=%PROGRAMDATA%\MantisDB
set CONFIG_DIR=%PROGRAMDATA%\MantisDB

echo Installing MantisDB...

REM Create directories
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%DATA_DIR%" mkdir "%DATA_DIR%"
if not exist "%CONFIG_DIR%" mkdir "%CONFIG_DIR%"

REM Copy binary
copy mantisdb.exe "%INSTALL_DIR%\"

REM Create default configuration
echo server: > "%CONFIG_DIR%\config.yaml"
echo   port: 8080 >> "%CONFIG_DIR%\config.yaml"
echo   admin_port: 8081 >> "%CONFIG_DIR%\config.yaml"
echo   host: "0.0.0.0" >> "%CONFIG_DIR%\config.yaml"
echo. >> "%CONFIG_DIR%\config.yaml"
echo database: >> "%CONFIG_DIR%\config.yaml"
echo   data_dir: "%DATA_DIR%" >> "%CONFIG_DIR%\config.yaml"
echo   cache_size: "256MB" >> "%CONFIG_DIR%\config.yaml"
echo   buffer_size: "64MB" >> "%CONFIG_DIR%\config.yaml"
echo   use_cgo: false >> "%CONFIG_DIR%\config.yaml"
echo   sync_writes: true >> "%CONFIG_DIR%\config.yaml"

REM Add to PATH
setx PATH "%PATH%;%INSTALL_DIR%" /M

echo Installation complete!
echo.
echo Configuration file: %CONFIG_DIR%\config.yaml
echo Data directory: %DATA_DIR%
echo.
echo To start MantisDB:
echo   mantisdb --config="%CONFIG_DIR%\config.yaml"
echo.
echo Admin dashboard will be available at: http://localhost:8081

pause
EOF

    # Create PowerShell installer as well
    cat > "$package_dir/install.ps1" << 'EOF'
# MantisDB PowerShell Installer

Write-Host "MantisDB Windows Installer" -ForegroundColor Green
Write-Host "=========================" -ForegroundColor Green

$InstallDir = "$env:ProgramFiles\MantisDB"
$DataDir = "$env:ProgramData\MantisDB"
$ConfigDir = "$env:ProgramData\MantisDB"

Write-Host "Installing MantisDB..." -ForegroundColor Yellow

# Create directories
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

# Copy binary
Copy-Item "mantisdb.exe" -Destination $InstallDir

# Create default configuration
$ConfigContent = @"
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "$DataDir"
  cache_size: "256MB"
  buffer_size: "64MB"
  use_cgo: false
  sync_writes: true

security:
  admin_token: ""
  enable_cors: false
  cors_origins: ["http://localhost:3000"]

logging:
  level: "info"
  format: "json"
  output: "stdout"
"@

$ConfigContent | Out-File -FilePath "$ConfigDir\config.yaml" -Encoding UTF8

# Add to PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$CurrentPath;$InstallDir", "Machine")
}

Write-Host "Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Configuration file: $ConfigDir\config.yaml"
Write-Host "Data directory: $DataDir"
Write-Host ""
Write-Host "To start MantisDB:"
Write-Host "  mantisdb --config=`"$ConfigDir\config.yaml`""
Write-Host ""
Write-Host "Admin dashboard will be available at: http://localhost:8081"

Read-Host "Press Enter to continue..."
EOF

    log_success "Windows installer created for $arch"
    return 0
}