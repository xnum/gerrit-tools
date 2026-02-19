#!/bin/bash
# gerrit-cli CLI 工具使用示例
# 這個腳本展示了 gerrit-cli 工具的各種用法

set -e

# 顏色輸出
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== gerrit-cli CLI 工具使用示例 ===${NC}\n"

# 檢查配置
if [ -z "$GERRIT_HTTP_URL" ]; then
    echo "請先設置環境變數："
    echo "  export GERRIT_HTTP_URL=\"https://gerrit.example.com\""
    echo "  export GERRIT_HTTP_USER=\"your-username\""
    echo "  export GERRIT_HTTP_PASSWORD=\"your-password\""
    exit 1
fi

echo -e "${GREEN}1. 查詢 Changes${NC}"
echo "列出開放的 changes："
echo "  gerrit-cli change list \"status:open\" --limit 5"
echo ""

echo -e "${GREEN}2. 獲取 Change 詳情${NC}"
echo "獲取特定 change 的詳細信息："
echo "  gerrit-cli change get 12345 --options CURRENT_REVISION --options MESSAGES"
echo ""

echo -e "${GREEN}3. 查看 Patchset Diff${NC}"
echo "列出 patchset 中的所有文件："
echo "  gerrit-cli patchset diff 12345 --list-files"
echo ""
echo "獲取特定文件的 diff："
echo "  gerrit-cli patchset diff 12345 --file src/main.go"
echo ""
echo "獲取所有文件的完整 diff："
echo "  gerrit-cli patchset diff 12345"
echo ""

echo -e "${GREEN}4. 列出評論${NC}"
echo "列出所有評論："
echo "  gerrit-cli comment list 12345"
echo ""
echo "僅列出未解決的評論："
echo "  gerrit-cli comment list 12345 --unresolved"
echo ""
echo "列出特定文件的評論："
echo "  gerrit-cli comment list 12345 --file src/main.go"
echo ""

echo -e "${GREEN}5. 發表審查${NC}"
echo "發表簡單的審查（+1）："
echo "  gerrit-cli review post 12345 --message \"Looks good!\" --vote 1"
echo ""
echo "發表帶內聯評論的審查："
echo "  gerrit-cli review post 12345 --message \"Some feedback\" --vote 0 \\"
echo "    --comment \"src/main.go:10:Consider using a constant\" \\"
echo "    --comment \"src/main.go:25:This function is too complex\""
echo ""

echo -e "${GREEN}6. JSON 輸出處理${NC}"
echo "使用 jq 處理 JSON 輸出："
echo "  gerrit-cli change list \"status:open\" | jq '.data[] | {number, subject}'"
echo ""
echo "檢查命令是否成功："
echo "  result=\$(gerrit-cli change get 12345)"
echo "  if [ \"\$(echo \$result | jq -r '.success')\" = \"true\" ]; then"
echo "    echo \"Success!\""
echo "  fi"
echo ""

echo -e "${GREEN}7. 在 Skill 中使用${NC}"
echo "創建一個簡單的 skill："
cat << 'EOF'
#!/bin/bash
# skill: gerrit-check-change
# 檢查 Gerrit change 的狀態

CHANGE_ID=$1

# 獲取 change 信息
change=$(gerrit-cli change get $CHANGE_ID --format json)

if [ "$(echo $change | jq -r '.success')" != "true" ]; then
    echo "Error: $(echo $change | jq -r '.error.message')"
    exit 1
fi

# 顯示基本信息
echo "Change: $CHANGE_ID"
echo "Subject: $(echo $change | jq -r '.data.subject')"
echo "Status: $(echo $change | jq -r '.data.status')"
echo "Owner: $(echo $change | jq -r '.data.owner.name')"

# 檢查未解決的評論
comments=$(gerrit-cli comment list $CHANGE_ID --unresolved --format json)
unresolved_count=$(echo $comments | jq '.data | length')
echo "Unresolved comments: $unresolved_count"
EOF
echo ""

echo -e "${GREEN}8. 文本格式輸出${NC}"
echo "如果需要人類可讀的輸出："
echo "  gerrit-cli change list \"status:open\" --format text"
echo ""

echo -e "${GREEN}9. 組合查詢${NC}"
echo "複雜查詢示例："
echo "  gerrit-cli change list \"status:open project:myproject branch:main owner:me\""
echo ""
echo "帶標籤過濾："
echo "  gerrit-cli change list \"status:open label:Code-Review=+2\""
echo ""
echo "可合併的 changes："
echo "  gerrit-cli change list \"status:open is:mergeable\""
echo ""

echo -e "${GREEN}10. 工作流示例${NC}"
echo "完整的審查工作流："
cat << 'EOF'
#!/bin/bash
CHANGE_ID=12345

# 1. 獲取 change 信息
echo "Fetching change info..."
gerrit-cli change get $CHANGE_ID

# 2. 列出文件
echo "Listing files..."
gerrit-cli patchset diff $CHANGE_ID --list-files

# 3. 查看特定文件的 diff
echo "Viewing main.go diff..."
gerrit-cli patchset diff $CHANGE_ID --file src/main.go

# 4. 檢查現有評論
echo "Checking existing comments..."
gerrit-cli comment list $CHANGE_ID --unresolved

# 5. 發表審查
echo "Posting review..."
gerrit-cli review post $CHANGE_ID \
  --message "Code review completed. Overall looks good with minor suggestions." \
  --vote 1 \
  --comment "src/main.go:42:Consider adding error handling here"
EOF
echo ""

echo -e "${BLUE}=== 完整文檔請參考 docs/GR_CLI_GUIDE.md ===${NC}"
