param(
  [string]$OutputPath = "excels/Demo.xlsx",
  [switch]$Force
)

$ErrorActionPreference = "Stop"

[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$ResolvedOutput = if ([System.IO.Path]::IsPathRooted($OutputPath)) {
  $OutputPath
} else {
  Join-Path $Root $OutputPath
}

function Wait-BeforeExit {
  if ($env:IOTAEXCEL_NO_PAUSE -eq "1") {
    return
  }
  Write-Host ""
  Read-Host "Press Enter to exit"
}

function Escape-Xml([string]$Value) {
  if ($null -eq $Value) { return "" }
  return [System.Security.SecurityElement]::Escape($Value)
}

function Get-ColName([int]$Index) {
  $name = ""
  while ($Index -gt 0) {
    $Index--
    $name = [char](65 + ($Index % 26)) + $name
    $Index = [math]::Floor($Index / 26)
  }
  return $name
}

function Write-TextFile {
  param([string]$Path, [string]$Content)

  $dir = Split-Path -Parent $Path
  if ($dir) {
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
  }
  [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

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

function New-StarterXlsx {
  param(
    [string]$Path,
    [string]$SheetName,
    [array]$Rows
  )

  if ((Test-Path $Path) -and -not $Force) {
    throw "Output already exists: $Path. Pass -Force to overwrite it."
  }

  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("iotaexcel-starter-" + [guid]::NewGuid().ToString("N"))
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $tmp "_rels") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $tmp "xl/_rels") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $tmp "xl/worksheets") | Out-Null

  try {
    $sharedMap = @{}
    $sharedList = [System.Collections.ArrayList]::new()

    $contentTypes = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<Types xmlns=`"http://schemas.openxmlformats.org/package/2006/content-types`">" +
      "<Default Extension=`"rels`" ContentType=`"application/vnd.openxmlformats-package.relationships+xml`"/>" +
      "<Default Extension=`"xml`" ContentType=`"application/xml`"/>" +
      "<Override PartName=`"/xl/workbook.xml`" ContentType=`"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml`"/>" +
      "<Override PartName=`"/xl/sharedStrings.xml`" ContentType=`"application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml`"/>" +
      "<Override PartName=`"/xl/worksheets/sheet1.xml`" ContentType=`"application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml`"/>" +
      "</Types>"
    Write-TextFile (Join-Path $tmp "[Content_Types].xml") $contentTypes

    $rootRels = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<Relationships xmlns=`"http://schemas.openxmlformats.org/package/2006/relationships`">" +
      "<Relationship Id=`"rId1`" Type=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument`" Target=`"xl/workbook.xml`"/>" +
      "</Relationships>"
    Write-TextFile (Join-Path $tmp "_rels/.rels") $rootRels

    $workbook = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<workbook xmlns=`"http://schemas.openxmlformats.org/spreadsheetml/2006/main`" xmlns:r=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships`">" +
      "<sheets><sheet name=`"$(Escape-Xml $SheetName)`" sheetId=`"1`" r:id=`"rId1`"/></sheets></workbook>"
    Write-TextFile (Join-Path $tmp "xl/workbook.xml") $workbook

    $workbookRels = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<Relationships xmlns=`"http://schemas.openxmlformats.org/package/2006/relationships`">" +
      "<Relationship Id=`"rId1`" Type=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet`" Target=`"worksheets/sheet1.xml`"/>" +
      "<Relationship Id=`"rId2`" Type=`"http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings`" Target=`"sharedStrings.xml`"/>" +
      "</Relationships>"
    Write-TextFile (Join-Path $tmp "xl/_rels/workbook.xml.rels") $workbookRels

    $rowsXml = ""
    for ($r = 0; $r -lt $Rows.Count; $r++) {
      $rowNo = $r + 1
      $cellsXml = ""
      $row = $Rows[$r]
      for ($c = 0; $c -lt $row.Count; $c++) {
        $value = [string]$row[$c]
        if ($value -eq "") { continue }

        $cellRef = "$(Get-ColName ($c + 1))$rowNo"
        $idx = Add-SharedString $sharedMap $sharedList $value
        $cellsXml += "<c r=`"$cellRef`" t=`"s`"><v>$idx</v></c>"
      }
      $rowsXml += "<row r=`"$rowNo`">$cellsXml</row>"
    }

    $worksheet = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<worksheet xmlns=`"http://schemas.openxmlformats.org/spreadsheetml/2006/main`"><sheetData>$rowsXml</sheetData></worksheet>"
    Write-TextFile (Join-Path $tmp "xl/worksheets/sheet1.xml") $worksheet

    $sstItems = ""
    foreach ($value in $sharedList) {
      $sstItems += "<si><t>$(Escape-Xml $value)</t></si>"
    }
    $sst = "<?xml version=`"1.0`" encoding=`"UTF-8`" standalone=`"yes`"?>" +
      "<sst xmlns=`"http://schemas.openxmlformats.org/spreadsheetml/2006/main`" count=`"$($sharedList.Count)`" uniqueCount=`"$($sharedList.Count)`">$sstItems</sst>"
    Write-TextFile (Join-Path $tmp "xl/sharedStrings.xml") $sst

    $outDir = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    $zipPath = [System.IO.Path]::ChangeExtension($Path, ".zip")
    if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
    if (Test-Path $Path) { Remove-Item $Path -Force }

    Compress-Archive -Path (Join-Path $tmp "*") -DestinationPath $zipPath -Force
    Move-Item $zipPath $Path -Force
  } finally {
    if (Test-Path $tmp) {
      Remove-Item $tmp -Recurse -Force
    }
  }
}

try {
  $starterRows = @(
    @("id#", "note", "ruleKey*", "allowedValues", "required", "example", "description"),
    @("int", "string", "string", "string", "bool", "string", "string"),
    @("all", "comment", "all", "all", "all", "all", "all"),
    @("唯一数字 ID", "注释列，导出时会忽略", "规则名称", "允许填写的值或格式", "是否必须理解或配置", "填写示例", "规则说明"),
    @("1", "表头行顺序", "headerRows", "字段名|字段类型|字段用途|字段注释|数据行", "true", "前 4 行是表头，第 5 行开始写数据", "每个 sheet 至少 5 行：第 1 行字段名，第 2 行类型，第 3 行用途，第 4 行注释，第 5 行开始为真实数据"),
    @("2", "字段名命名", "fieldName", "^[A-Za-z_][A-Za-z0-9_]*$", "true", "itemId 或 name", "字段名必须是 ASCII 标识符，便于生成多语言读取代码"),
    @("3", "Key 标记", "keyMarker", "字段名末尾加 #", "true", "id#", "每个 sheet 必须且只能有一个 key 字段；key 隐含必填和唯一"),
    @("4", "必填标记", "requiredMarker", "字段名末尾加 *", "false", "name*", "带 * 的字段在数据行中不能为空；key 字段自动视为必填"),
    @("5", "唯一标记", "uniqueMarker", "字段名末尾加 !", "false", "code!", "带 ! 的非空值不能重复；key 字段自动视为唯一"),
    @("6", "基础类型", "primitiveTypes", "bool|int|int32|int64|float|double|string|bytes|datetime", "true", "int32", "第 2 行填写字段类型；不支持的类型会导致当前工作簿校验失败"),
    @("7", "布尔值写法", "boolValues", "true|false|1|0", "false", "true", "bool 字段支持 true、false、1、0，大小写按当前解析规则处理"),
    @("8", "时间写法", "datetimeFormat", "YYYY-MM-DD HH:mm:ss", "false", "2026-07-18 10:00:00", "datetime 必须使用固定格式，导出为秒级时间戳"),
    @("9", "数组写法", "arrayType", "array<T>，值用 | 分隔", "false", "weapon|rare|starter", "array<string> 等数组类型在单元格中使用竖线分隔多个值"),
    @("10", "Map 写法", "mapType", "map<K,V>，值用 key:value|key:value", "false", "atk:10|level:2", "map<string,int> 等 map 类型在单元格中使用冒号分隔键值，用竖线分隔条目"),
    @("11", "引用写法", "refType", "ref<SheetName>", "false", "ref<Item>", "ref<T> 表示引用目标 sheet 的 key；开启引用检查时会验证目标表和值是否存在"),
    @("12", "目标用途", "usageValues", "client|server|all|comment", "true", "all", "第 3 行填写字段用途，用于控制字段导出到客户端、服务器、两端，或仅作为备注列"),
    @("13", "用途别名", "usageAliases", "c|cli|s|srv|cs|common|note|remark|ignore|skip", "false", "srv", "用途大小写不敏感，并支持常用别名；comment 用途不能和导出用途混用"),
    @("14", "备注列", "commentUsage", "comment", "false", "note 列用途写 comment", "用途为 comment 的字段不会写入 .bytes，适合放策划说明、来源或维护备注"),
    @("15", "命名规则", "identifierNames", "文件名、sheet 名、ref<T> 目标名都必须是 ASCII 标识符", "true", "Demo.xlsx / Item / ref<Item>", "文件名去掉 .xlsx 后、sheet 名、ref<T> 中的目标名都要满足标识符规则")
  )

  New-StarterXlsx -Path $ResolvedOutput -SheetName "Demo" -Rows $starterRows
  Write-Host "Generated starter Excel: $ResolvedOutput"
} finally {
  Wait-BeforeExit
}
