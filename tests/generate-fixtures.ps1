$ErrorActionPreference = "Stop"

# 该脚本用于生成集成测试所需的 .xlsx fixture。
# 为了避免依赖本机安装 Excel 或第三方库，这里直接写入 XLSX 所需的最小 XML 结构，
# 再通过 Compress-Archive 打包成 zip 并重命名为 .xlsx。
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$ExcelRoot = Join-Path $Root "testdata/excels"

function Wait-BeforeExit {
  if ($env:IOTAEXCEL_NO_PAUSE -eq "1") {
    return
  }
  Write-Host ""
  Read-Host "Press Enter to exit"
}

# 转义 XML 文本节点或属性中的特殊字符，避免测试数据破坏生成的 worksheet XML。
function Escape-Xml([string]$value) {
  if ($null -eq $value) { return "" }
  return [System.Security.SecurityElement]::Escape($value)
}

# 把从 1 开始的列号转换为 Excel 列名，例如 1=A、26=Z、27=AA。
function Get-ColName([int]$index) {
  $name = ""
  while ($index -gt 0) {
    $index--
    $name = [char](65 + ($index % 26)) + $name
    $index = [math]::Floor($index / 26)
  }
  return $name
}

# 把字符串加入 sharedStrings 表，并返回它的索引。
# 默认 fixture 使用 sharedStrings，以覆盖最常见的 Excel 字符串存储方式。
function Add-SharedString {
  param(
    [hashtable]$Map,
    [System.Collections.ArrayList]$List,
    [string]$Value
  )
  if (-not $Map.ContainsKey($Value)) {
    $Map[$Value] = $List.Count
    [void]$List.Add($Value)
  }
  return $Map[$Value]
}

# 写入 UTF-8 无 BOM 文本文件。
# PowerShell 5.1 默认编码容易受系统区域影响，因此这里显式指定编码。
function Write-TextFile {
  param([string]$Path, [string]$Content)
  $dir = Split-Path -Parent $Path
  if ($dir) {
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
  }
  [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

# 创建一个最小可读的 .xlsx 文件。
# Sheets 参数是 sheet 名和二维行数据；InlineStrings 用于生成 inlineStr 单元格，
# 以便测试 xlsx reader 对 sharedStrings 之外格式的兼容性。
function New-Xlsx {
  param(
    [string]$OutputPath,
    [array]$Sheets,
    [switch]$InlineStrings
  )

  # 先在临时目录中组装 XLSX 内部文件，最后整体压缩，避免半成品污染 testdata。
  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("iotaexcel-fixture-" + [guid]::NewGuid().ToString("N"))
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $tmp "_rels") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $tmp "xl/_rels") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $tmp "xl/worksheets") | Out-Null

  # sharedMap 用于去重字符串，sharedList 按索引顺序写入 xl/sharedStrings.xml。
  $sharedMap = @{}
  $sharedList = [System.Collections.ArrayList]::new()

  # [Content_Types].xml 声明 workbook、worksheet、sharedStrings 等部件的 MIME 类型。
  $sheetContentTypes = ""
  for ($i = 0; $i -lt $Sheets.Count; $i++) {
    $sheetNo = $i + 1
    $sheetContentTypes += "<Override PartName=`"/xl/worksheets/sheet$sheetNo.xml`" ContentType=`"application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml`"/>"
  }

  $contentTypes = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
    "<Types xmlns=`"http://schemas.openxmlformats.org/package/2006/content-types`">" +
    "<Default Extension=`"rels`" ContentType=`"application/vnd.openxmlformats-package.relationships+xml`"/>" +
    "<Default Extension=`"xml`" ContentType=`"application/xml`"/>" +
    "<Override PartName=`"/xl/workbook.xml`" ContentType=`"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml`"/>" +
    "<Override PartName=`"/xl/sharedStrings.xml`" ContentType=`"application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml`"/>" +
    $sheetContentTypes +
    "</Types>"
  Write-TextFile (Join-Path $tmp "[Content_Types].xml") $contentTypes

  # 根 relationship 告诉读取器真正的 workbook.xml 位于 xl/workbook.xml。
  $rootRels = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
    "<Relationships xmlns=`"http://schemas.openxmlformats.org/package/2006/relationships`">" +
    "<Relationship Id=`"rId1`" Type=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument`" Target=`"xl/workbook.xml`"/>" +
    "</Relationships>"
  Write-TextFile (Join-Path $tmp "_rels/.rels") $rootRels

  # workbook.xml 保存 sheet 名；workbook.xml.rels 保存 sheet id 到 worksheet XML 的映射。
  $workbookSheets = ""
  $workbookRels = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
    "<Relationships xmlns=`"http://schemas.openxmlformats.org/package/2006/relationships`">"

  # 逐个 sheet 写 worksheet XML。空字符串单元格不写入 XML，用来模拟真实 Excel 的稀疏单元格。
  for ($i = 0; $i -lt $Sheets.Count; $i++) {
    $sheetNo = $i + 1
    $name = Escape-Xml $Sheets[$i].Name
    $workbookSheets += "<sheet name=`"$name`" sheetId=`"$sheetNo`" r:id=`"rId$sheetNo`"/>"
    $workbookRels += "<Relationship Id=`"rId$sheetNo`" Type=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet`" Target=`"worksheets/sheet$sheetNo.xml`"/>"
  }
  $sharedRid = $Sheets.Count + 1
  $workbookRels += "<Relationship Id=`"rId$sharedRid`" Type=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings`" Target=`"sharedStrings.xml`"/>"
  $workbookRels += "</Relationships>"

  $workbook = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
    "<workbook xmlns=`"http://schemas.openxmlformats.org/spreadsheetml/2006/main`" xmlns:r=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships`">" +
    "<sheets>$workbookSheets</sheets></workbook>"
  Write-TextFile (Join-Path $tmp "xl/workbook.xml") $workbook
  Write-TextFile (Join-Path $tmp "xl/_rels/workbook.xml.rels") $workbookRels

  for ($i = 0; $i -lt $Sheets.Count; $i++) {
    $rowsXml = ""
    $rows = $Sheets[$i].Rows
    for ($r = 0; $r -lt $rows.Count; $r++) {
      $rowNo = $r + 1
      $cellsXml = ""
      $row = $rows[$r]
      for ($c = 0; $c -lt $row.Count; $c++) {
        $value = [string]$row[$c]
        if ($value -eq "") { continue }
        $cellRef = "$(Get-ColName ($c + 1))$rowNo"
        $escaped = Escape-Xml $value
        if ($InlineStrings) {
          $cellsXml += "<c r=`"$cellRef`" t=`"inlineStr`"><is><t>$escaped</t></is></c>"
        } else {
          $idx = Add-SharedString $sharedMap $sharedList $value
          $cellsXml += "<c r=`"$cellRef`" t=`"s`"><v>$idx</v></c>"
        }
      }
      $rowsXml += "<row r=`"$rowNo`">$cellsXml</row>"
    }
    $worksheet = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<worksheet xmlns=`"http://schemas.openxmlformats.org/spreadsheetml/2006/main`"><sheetData>$rowsXml</sheetData></worksheet>"
    Write-TextFile (Join-Path $tmp "xl/worksheets/sheet$($i + 1).xml") $worksheet
  }

  # sharedStrings.xml 保存所有非 inlineStr 字符串。
  $sstItems = ""
  foreach ($value in $sharedList) {
    $sstItems += "<si><t>$(Escape-Xml $value)</t></si>"
  }
  $sst = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
    "<sst xmlns=`"http://schemas.openxmlformats.org/spreadsheetml/2006/main`" count=`"$($sharedList.Count)`" uniqueCount=`"$($sharedList.Count)`">$sstItems</sst>"
  Write-TextFile (Join-Path $tmp "xl/sharedStrings.xml") $sst

  # Compress-Archive 要求输出扩展名为 .zip，因此先写 zip，再重命名为 .xlsx。
  $outDir = Split-Path -Parent $OutputPath
  New-Item -ItemType Directory -Force -Path $outDir | Out-Null
  if (Test-Path $OutputPath) { Remove-Item $OutputPath -Force }
  $zipPath = [System.IO.Path]::ChangeExtension($OutputPath, ".zip")
  if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
  Compress-Archive -Path (Join-Path $tmp "*") -DestinationPath $zipPath -Force
  Move-Item $zipPath $OutputPath -Force
  Remove-Item $tmp -Recurse -Force
}

function Sheet($Name, $Rows) {
  return @{ Name = $Name; Rows = $Rows }
}

try {
# 每次重新生成 fixture 都清空旧目录，确保测试数据完全由当前脚本定义。
Remove-Item $ExcelRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $ExcelRoot | Out-Null

# 主合法 fixture：覆盖 key、用途别名、基础类型、负整数、日期、array、map、comment、空行跳过。
$validItem = @(
  @("id#", "name!", "label*", "enabled", "score", "big", "ratio", "price", "createdAt", "tags", "attrs", "note", "serverOnly", "clientOnly!"),
  @("int", "string", "string", "bool", "int32", "int64", "float", "double", "datetime", "array<string>", "map<string,int>", "string", "string", "string"),
  @("all", "all", "all", "c", "client", "server", "all", "all", "all", "all", "all", "comment", "srv", "CLIENT"),
  @("unique id", "name", "required label", "enabled", "negative int", "long integer", "float value", "double value", "created time", "tags", "attrs", "note", "server field", "unique client field"),
  @("1001", "Sword", "Weapon", "true", "-12", "-900719925474", "1.5", "3.14159", "2026-07-10 18:47:00", "weapon|rare", "atk:10|level:2", "internal note", "srv value", "cli value"),
  @("", "", "", "", "", "", "", "", "", "", "", "", "", ""),
  @("1002", "Shield", "Armor", "0", "0", "42", "0", "1.25", "2026-07-11 09:30:00", "armor|common", "def:20|level:1", "remark", "srv2", "cli2")
)

# 第二个合法 sheet：覆盖 ref<Item> 引用和字符串 key。
$validHero = @(
  @("id#", "itemRef", "name", "active"),
  @("string", "ref<Item>", "string", "bool"),
  @("all", "all", "all", "all"),
  @("hero id", "ref Item key", "name", "active"),
  @("hero_001", "1001", "Alice", "1"),
  @("hero_002", "1002", "Bob", "false")
)

# Config.xlsx 是主要集成测试输入，包含 Item/Hero 两张表。
New-Xlsx -OutputPath (Join-Path $ExcelRoot "valid/Config.xlsx") -Sheets @(
  (Sheet "Item" $validItem),
  (Sheet "Hero" $validHero)
)

# InlineStrings.xlsx 用于验证 inlineStr 单元格读取。
New-Xlsx -OutputPath (Join-Path $ExcelRoot "valid/InlineStrings.xlsx") -InlineStrings -Sheets @(
  (Sheet "Inline" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "inline")
  ))
)

# Defaults.xlsx 用于验证空值和非法值是否按类型回退默认值。
New-Xlsx -OutputPath (Join-Path $ExcelRoot "valid/Defaults.xlsx") -Sheets @(
  (Sheet "Defaults" @(
    @("id#", "badBool", "badInt", "badDate", "emptyString", "emptyArray", "emptyRef"),
    @("int", "bool", "int32", "datetime", "string", "array<string>", "ref<Item>"),
    @("all", "all", "all", "all", "all", "all", "all"),
    @("ID", "default false", "default zero", "default year one", "default empty string", "default null", "default null"),
    @("1", "yes", "abc", "2026/07/10", "", "", "")
  ))
)

# invalid 目录中的 fixture 分别覆盖 schema 校验应该拒绝的错误输入。
New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/MissingKey.xlsx") -Sheets @(
  (Sheet "MissingKey" @(
    @("id", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/DuplicateKey.xlsx") -Sheets @(
  (Sheet "DuplicateKey" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "a"),
    @("1", "b")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/InvalidFieldName.xlsx") -Sheets @(
  (Sheet "InvalidFieldName" @(
    @("1id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/InvalidUsage.xlsx") -Sheets @(
  (Sheet "InvalidUsage" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "unknown"),
    @("ID", "name"),
    @("1", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/InvalidType.xlsx") -Sheets @(
  (Sheet "InvalidType" @(
    @("id#", "name"),
    @("int", "vector3"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/CommentKey.xlsx") -Sheets @(
  (Sheet "CommentKey" @(
    @("id#", "name"),
    @("int", "string"),
    @("comment", "all"),
    @("ID", "name"),
    @("1", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/EmptyKey.xlsx") -Sheets @(
  (Sheet "EmptyKey" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/InvalidFileName.xlsx") -Sheets @(
  (Sheet "ValidSheet" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "a")
  ))
)
Rename-Item (Join-Path $ExcelRoot "invalid/InvalidFileName.xlsx") (Join-Path $ExcelRoot "invalid/Invalid File.xlsx")

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/InvalidSheetName.xlsx") -Sheets @(
  (Sheet "Invalid Sheet" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "名称"),
    @("1", "a")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "invalid/RefMissing.xlsx") -Sheets @(
  (Sheet "Hero" @(
    @("id#", "itemRef"),
    @("int", "ref<Item>"),
    @("all", "all"),
    @("ID", "missing ref table"),
    @("1", "1001")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "nested/SubConfig.xlsx") -Sheets @(
  (Sheet "SubConfig" @(
    @("id#", "name"),
    @("int", "string"),
    @("all", "all"),
    @("ID", "name"),
    @("1", "nested")
  ))
)

New-Xlsx -OutputPath (Join-Path $ExcelRoot "~`$Temp.xlsx") -Sheets @(
  (Sheet "Temp" @(
    @("id#"),
    @("int"),
    @("all"),
    @("ID"),
    @("1")
  ))
)

Write-Host "Generated Excel fixtures under $ExcelRoot"
} finally {
  Wait-BeforeExit
}
