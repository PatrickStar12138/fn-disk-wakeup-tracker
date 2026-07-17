import fs from 'node:fs'
import path from 'node:path'

const root = process.argv[2]
const ignored = new Set(['.git', '.cache', 'node_modules', 'dist', 'build', 'release'])

/** collectFiles 递归收集需要检查的源码文件，并跳过依赖和构建产物。 */
function collectFiles(directory, output = []) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (ignored.has(entry.name)) continue
    const absolute = path.join(directory, entry.name)
    if (entry.isDirectory()) collectFiles(absolute, output)
    else if (/\.(go|ts|vue|sh)$/.test(entry.name) || absolute.includes(`${path.sep}packaging${path.sep}fnos${path.sep}cmd${path.sep}`)) output.push(absolute)
  }
  return output
}

/** commentBlock 返回声明前连续注释块，用于确认存在准确中文说明。 */
function commentBlock(lines, index, style) {
  const found = []
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    const value = lines[cursor].trim()
    if (value === '') { if (found.length === 0) continue; break }
    const isComment = style === 'shell' ? value.startsWith('#') : value.startsWith('//') || value.startsWith('/*') || value.startsWith('*') || value.endsWith('*/')
    if (!isComment) break
    found.unshift(value)
  }
  return found.join('\n')
}

const failures = []
for (const file of collectFiles(root)) {
  const relative = path.relative(root, file)
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/)
  const shell = file.endsWith('.sh') || relative.includes(`packaging${path.sep}fnos${path.sep}cmd${path.sep}`)
  lines.forEach((line, index) => {
    let declaration = false
    if (file.endsWith('.go')) declaration = /^\s*(func\s+|type\s+\w+|var\s+\(|const\s+\()/.test(line)
    else if (file.endsWith('.ts') || file.endsWith('.vue')) declaration = /^\s*(export\s+)?(async\s+)?function\s+|^\s*export\s+(type|interface)\s+|^\s*const\s+\w+\s*=\s*computed\b/.test(line)
    else if (shell) declaration = /^\s*[A-Za-z_][A-Za-z0-9_]*\(\)\s*\{/.test(line)
    if (!declaration) return
    const block = commentBlock(lines, index, shell ? 'shell' : 'code')
    if (!/\p{Script=Han}/u.test(block)) failures.push(`${relative}:${index + 1} 缺少紧邻声明的中文注释`)
  })
  lines.forEach((line, index) => {
    if (/\bTODO\b/.test(line) && !/v\d+\.\d+\.\d+|触发条件|真机/.test(line)) failures.push(`${relative}:${index + 1} TODO 缺少原因、目标版本或触发条件`)
  })
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  console.error(`中文注释检查失败：${failures.length} 项`)
  process.exit(1)
}
console.log('中文注释检查通过')

