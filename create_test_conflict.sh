#!/bin/bash

# Script to help create a conflict scenario for testing

echo "🔧 Creating test conflict scenario..."

# Check if we're in the right directory
if [ ! -f "notion-md-sync" ]; then
    echo "❌ Please run this script in the directory containing your notion-md-sync binary"
    exit 1
fi

if [ ! -f "config.yaml" ] && [ ! -f ".env" ]; then
    echo "❌ Please make sure you have config.yaml or .env file in this directory"
    exit 1
fi

echo "📋 Instructions to create a conflict:"
echo ""
echo "1. Find a markdown file that's already synced to Notion (has notion_id in frontmatter)"
echo "2. Edit the file locally - add this content:"
echo ""
echo "   **LOCAL TEST**: Added locally at $(date)"
echo "   - Local change 1"
echo "   - Local change 2"
echo ""
echo "3. Go to the corresponding Notion page and add different content:"
echo "   **REMOTE TEST**: Added in Notion"
echo "   - Remote change 1" 
echo "   - Remote change 2"
echo ""
echo "4. Run: ./notion-md-sync sync --direction bidirectional"
echo ""
echo "5. You should see the conflict resolution prompt!"
echo ""
echo "📝 Pro tip: Make sure your config.yaml has:"
echo "   sync:"
echo "     direction: bidirectional"
echo "     conflict_resolution: diff"
echo ""
echo "✅ Ready to test conflict resolution!"