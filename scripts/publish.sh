#!/bin/bash

# Drop Docker Publish Script
# Handles version bumping, Docker publishing (multi-platform), and git tagging

set -e

# Parse arguments
DRY_RUN=false
SKIP_TESTS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --dry-run      Run without pushing to Docker Hub or creating git tags"
            echo "  --skip-tests   Skip running tests (use with caution)"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROJECT_FILE="$PROJECT_ROOT/project.json"

# Check if project.json exists
if [ ! -f "$PROJECT_FILE" ]; then
    echo -e "${RED}Error: project.json not found at $PROJECT_FILE${NC}"
    exit 1
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: jq is required but not installed.${NC}"
    echo "Install with: sudo apt install jq (Ubuntu) or brew install jq (Mac)"
    exit 1
fi

# Check if docker buildx is available
if ! docker buildx version &> /dev/null; then
    echo -e "${RED}Error: docker buildx is required but not available.${NC}"
    echo "Make sure you have Docker Desktop or buildx plugin installed."
    exit 1
fi

# Read project info
PROJECT_NAME=$(jq -r '.name' "$PROJECT_FILE")
CURRENT_VERSION=$(jq -r '.version' "$PROJECT_FILE")
DESCRIPTION=$(jq -r '.description' "$PROJECT_FILE")
DOCKER_IMAGE=$(jq -r '.docker.image' "$PROJECT_FILE")
DOCKER_USERNAME=$(jq -r '.docker.username' "$PROJECT_FILE")

# Parse current version
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

display_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}            ${BOLD}Drop Docker Publish Script${NC}                       ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    if [ "$DRY_RUN" = true ]; then
        echo -e "                    ${YELLOW}[ DRY RUN MODE ]${NC}"
    fi
    echo ""
}

display_info() {
    echo -e "${BOLD}Project Information:${NC}"
    echo -e "  ${BLUE}Name:${NC}           $PROJECT_NAME"
    echo -e "  ${BLUE}Description:${NC}    $DESCRIPTION"
    echo -e "  ${BLUE}Current Version:${NC} ${YELLOW}v$CURRENT_VERSION${NC}"
    echo -e "  ${BLUE}Docker Image:${NC}   $DOCKER_IMAGE"
    echo ""
}

calculate_version() {
    local bump_type=$1
    case $bump_type in
        1) NEW_VERSION="$MAJOR.$MINOR.$((PATCH + 1))" ;;
        2) NEW_VERSION="$MAJOR.$((MINOR + 1)).0" ;;
        3) NEW_VERSION="$((MAJOR + 1)).0.0" ;;
        4) NEW_VERSION="$CURRENT_VERSION" ;;
        *)
            echo -e "${RED}Invalid option${NC}"
            exit 1
            ;;
    esac
}

run_tests() {
    echo ""
    echo -e "${BOLD}Running Health Check...${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    cd "$PROJECT_ROOT"

    echo -e "${CYAN}Building and starting container...${NC}"
    docker compose build
    docker compose up -d
    sleep 3

    if curl -sf http://localhost:8802/api/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Health check passed!${NC}"
        docker compose down
        return 0
    else
        echo -e "${RED}✗ Health check failed! Aborting publish.${NC}"
        docker compose down
        return 1
    fi
}

update_version() {
    echo ""
    echo -e "${BOLD}Updating version in project.json...${NC}"

    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[DRY RUN] Would update version to $NEW_VERSION${NC}"
        return 0
    fi

    jq ".version = \"$NEW_VERSION\"" "$PROJECT_FILE" > "$PROJECT_FILE.tmp" && mv "$PROJECT_FILE.tmp" "$PROJECT_FILE"
    echo -e "${GREEN}✓ Version updated to $NEW_VERSION${NC}"
}

setup_buildx() {
    echo ""
    echo -e "${BOLD}Setting up Docker Buildx...${NC}"

    if ! docker buildx inspect drop-builder &> /dev/null; then
        echo -e "${CYAN}Creating multi-platform builder...${NC}"
        docker buildx create --name drop-builder --use --bootstrap
    else
        docker buildx use drop-builder
    fi

    echo -e "${GREEN}✓ Buildx builder ready${NC}"
}

publish_docker() {
    echo ""
    echo -e "${BOLD}Building & Pushing Multi-Platform Docker Image...${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}Platforms: linux/amd64, linux/arm64${NC}"
    echo ""

    cd "$PROJECT_ROOT"

    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[DRY RUN] Would execute:${NC}"
        echo -e "  docker buildx build \\"
        echo -e "    --platform linux/amd64,linux/arm64 \\"
        echo -e "    -t $DOCKER_IMAGE:$NEW_VERSION \\"
        echo -e "    -t $DOCKER_IMAGE:latest \\"
        echo -e "    --push ."
        echo ""
        echo -e "${YELLOW}[DRY RUN] Skipping Docker build and push${NC}"
        return 0
    fi

    if ! docker info 2>/dev/null | grep -q "Username"; then
        echo -e "${YELLOW}Please log in to Docker Hub:${NC}"
        docker login
    fi

    setup_buildx

    echo ""
    echo -e "${CYAN}Building and pushing $DOCKER_IMAGE:$NEW_VERSION ...${NC}"
    echo ""

    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        -t "$DOCKER_IMAGE:$NEW_VERSION" \
        -t "$DOCKER_IMAGE:latest" \
        --push \
        .

    echo ""
    echo -e "${GREEN}✓ Docker images published successfully!${NC}"
    echo -e "  ${BLUE}Platforms:${NC} linux/amd64, linux/arm64"
    echo -e "  ${BLUE}Tags:${NC} $NEW_VERSION, latest"
}

create_git_tag() {
    echo ""
    echo -e "${BOLD}Creating Git Tag...${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    cd "$PROJECT_ROOT"

    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}[DRY RUN] Would commit version bump and create git tag v$NEW_VERSION${NC}"
        echo -e "${YELLOW}[DRY RUN] Skipping git operations${NC}"
        return 0
    fi

    if [ -n "$(git status --porcelain)" ]; then
        echo -e "${YELLOW}Uncommitted changes detected. Committing version bump...${NC}"
        git add project.json
        git commit -m "(chore): bump version to v$NEW_VERSION"
    fi

    if git rev-parse "v$NEW_VERSION" >/dev/null 2>&1; then
        echo -e "${YELLOW}Tag v$NEW_VERSION already exists.${NC}"
        read -p "Do you want to delete and recreate it? (y/N): " recreate
        if [[ $recreate =~ ^[Yy]$ ]]; then
            git tag -d "v$NEW_VERSION"
            git push origin --delete "v$NEW_VERSION" 2>/dev/null || true
        else
            echo -e "${YELLOW}Skipping git tag creation.${NC}"
            return 0
        fi
    fi

    git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"
    echo -e "${GREEN}✓ Git tag v$NEW_VERSION created${NC}"

    read -p "Push tag to remote? (Y/n): " push_tag
    if [[ ! $push_tag =~ ^[Nn]$ ]]; then
        git push origin "v$NEW_VERSION"
        echo -e "${GREEN}✓ Tag pushed to remote${NC}"
    fi
}

display_summary() {
    echo ""
    if [ "$DRY_RUN" = true ]; then
        echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${CYAN}║${NC}              ${BOLD}${YELLOW}Dry Run Complete!${NC}                          ${CYAN}║${NC}"
        echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    else
        echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${CYAN}║${NC}                  ${BOLD}${GREEN}Publish Complete!${NC}                        ${CYAN}║${NC}"
        echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    fi
    echo ""
    echo -e "${BOLD}Summary:${NC}"
    echo -e "  ${BLUE}Version:${NC}      v$CURRENT_VERSION → ${GREEN}v$NEW_VERSION${NC}"
    echo -e "  ${BLUE}Docker:${NC}       $DOCKER_IMAGE:$NEW_VERSION"
    echo -e "  ${BLUE}Platforms:${NC}    linux/amd64, linux/arm64"
    echo -e "  ${BLUE}Git Tag:${NC}      v$NEW_VERSION"
    echo ""
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}This was a dry run. No changes were made.${NC}"
        echo -e "${YELLOW}Run without --dry-run to publish for real.${NC}"
    else
        echo -e "${BOLD}Docker Pull Command:${NC}"
        echo -e "  ${CYAN}docker pull $DOCKER_IMAGE:$NEW_VERSION${NC}"
        echo -e "  ${CYAN}docker pull $DOCKER_IMAGE:latest${NC}"
    fi
    echo ""
}

main() {
    display_header
    display_info

    echo -e "${BOLD}Select version bump type:${NC}"
    echo ""
    echo -e "  ${YELLOW}1)${NC} Patch  ${BLUE}(v$MAJOR.$MINOR.$((PATCH + 1)))${NC}  - Bug fixes, minor changes"
    echo -e "  ${YELLOW}2)${NC} Minor  ${BLUE}(v$MAJOR.$((MINOR + 1)).0)${NC}  - New features, backward compatible"
    echo -e "  ${YELLOW}3)${NC} Major  ${BLUE}(v$((MAJOR + 1)).0.0)${NC}  - Breaking changes"
    echo -e "  ${YELLOW}4)${NC} Same   ${BLUE}(v$CURRENT_VERSION)${NC}  - Republish current version"
    echo -e "  ${YELLOW}5)${NC} Cancel"
    echo ""

    read -p "Enter choice [1-5]: " choice

    if [ "$choice" == "5" ]; then
        echo -e "${YELLOW}Cancelled.${NC}"
        exit 0
    fi

    calculate_version "$choice"

    echo ""
    if [ "$NEW_VERSION" == "$CURRENT_VERSION" ]; then
        echo -e "${BOLD}Republish version:${NC} ${YELLOW}v$CURRENT_VERSION${NC}"
    else
        echo -e "${BOLD}Version change:${NC} v$CURRENT_VERSION → ${GREEN}v$NEW_VERSION${NC}"
    fi
    echo ""
    echo -e "${BOLD}This will:${NC}"
    echo -e "  1. Run health check"
    echo -e "  2. Update version in project.json"
    echo -e "  3. Build multi-platform Docker image (amd64 + arm64)"
    echo -e "  4. Push to Docker Hub: $DOCKER_IMAGE:$NEW_VERSION, :latest"
    echo -e "  5. Create git tag: v$NEW_VERSION"
    echo ""

    read -p "Continue? (y/N): " confirm
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Cancelled.${NC}"
        exit 0
    fi

    if [ "$SKIP_TESTS" = true ]; then
        echo -e "${YELLOW}Skipping tests (--skip-tests flag)${NC}"
    else
        run_tests || exit 1
    fi

    if [ "$NEW_VERSION" != "$CURRENT_VERSION" ]; then
        update_version
    fi

    publish_docker
    create_git_tag
    display_summary
}

main "$@"
