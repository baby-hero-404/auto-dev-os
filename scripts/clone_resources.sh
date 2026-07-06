#!/bin/bash

# Ensure we're in the project root by resolving the script path
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$PROJECT_ROOT/resources"
cd "$PROJECT_ROOT/resources" || exit

clone_repo() {
    local url=$1
    local name=$(basename "$url" .git)
    if [ ! -d "$name" ]; then
        echo "Cloning $name..."
        git clone "$url"
    else
        echo "Repository $name already exists. Fetching latest changes..."
        git -C "$name" pull
    fi
}

clone_repo "https://github.com/multica-ai/multica.git"
clone_repo "https://github.com/openclaw/openclaw.git"
clone_repo "https://github.com/ai-sdlc-framework/ai-sdlc.git"
clone_repo "https://github.com/decolua/9router.git"
clone_repo "https://github.com/Fission-AI/OpenSpec.git"
clone_repo "https://github.com/rohitg00/agentmemory.git"
clone_repo "https://github.com/obra/superpowers.git"
clone_repo "https://github.com/nousresearch/hermes-agent.git"
clone_repo "https://github.com/Alishahryar1/free-claude-code.git"
clone_repo "https://github.com/sunshine12396/prompt_base.git"
clone_repo "https://github.com/sickn33/antigravity-awesome-skills.git"
clone_repo "https://github.com/sunshine12396/llm-key-manager.git"
clone_repo "https://github.com/headroomlabs-ai/headroom.git"
clone_repo "https://github.com/Aider-AI/aider.git"
