import { existsSync, readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { parseDocument } from 'yaml'

const bundleRoot = path.resolve(process.argv[2] ?? 'okf')
const errors = []
const counts = { concepts: 0, indexes: 0, logs: 0 }

function fail(file, message) {
  errors.push(`${path.relative(bundleRoot, file) || path.basename(file)}: ${message}`)
}

function markdownFiles(directory) {
  return readdirSync(directory, { withFileTypes: true })
    .flatMap((entry) => {
      const target = path.join(directory, entry.name)
      if (entry.isDirectory()) return markdownFiles(target)
      return entry.isFile() && entry.name.endsWith('.md') ? [target] : []
    })
    .sort()
}

function frontmatter(text, file) {
  if (!text.startsWith('---\n')) return undefined
  const closing = text.indexOf('\n---\n', 4)
  if (closing < 0) {
    fail(file, 'unterminated YAML frontmatter')
    return undefined
  }

  const document = parseDocument(text.slice(4, closing), { uniqueKeys: true })
  for (const error of document.errors) fail(file, `invalid YAML frontmatter: ${error.message}`)
  const value = document.toJS()
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    fail(file, 'frontmatter must be a YAML mapping')
    return undefined
  }
  return value
}

function hasExplicitOffset(value) {
  return (
    typeof value === 'string' &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) &&
    !Number.isNaN(Date.parse(value))
  )
}

function isExternalOrDescriptor(value) {
  return /^[a-z][a-z0-9+.-]*:/i.test(value) || /\s/.test(value)
}

function resolveReference(file, value) {
  if (value.startsWith('/')) return path.join(bundleRoot, value.slice(1))
  return path.resolve(path.dirname(file), value)
}

function validatePath(file, value, label) {
  if (typeof value !== 'string' || value.length === 0) {
    fail(file, `${label} must be a non-empty string`)
    return
  }
  if (isExternalOrDescriptor(value)) return
  if (!existsSync(resolveReference(file, value))) fail(file, `${label} does not resolve: ${value}`)
}

function validateActorEvent(file, value, label, { requireAt = false } = {}) {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    fail(file, `${label} must be a mapping`)
    return
  }
  if (typeof value.by !== 'string' || value.by.length === 0) fail(file, `${label}.by is required`)
  if (requireAt && value.at === undefined) fail(file, `${label}.at is required`)
  if (value.at !== undefined && !hasExplicitOffset(value.at)) {
    fail(file, `${label}.at must be an ISO 8601 datetime with an explicit UTC offset`)
  }
}

function validateConcept(file, text, metadata) {
  counts.concepts += 1
  if (!metadata) {
    fail(file, 'concept is missing YAML frontmatter')
    return
  }
  if (typeof metadata.type !== 'string' || metadata.type.trim().length === 0) {
    fail(file, 'concept requires a non-empty type')
  }
  if (
    metadata.status !== undefined &&
    !['draft', 'stable', 'deprecated'].includes(metadata.status)
  ) {
    fail(file, 'status must be draft, stable, or deprecated')
  }
  if (metadata.stale_after !== undefined && !hasExplicitOffset(metadata.stale_after)) {
    fail(file, 'stale_after must be an ISO 8601 datetime with an explicit UTC offset')
  }
  if (metadata.generated !== undefined) validateActorEvent(file, metadata.generated, 'generated')
  if (metadata.resource !== undefined) validatePath(file, metadata.resource, 'resource')

  const verified =
    metadata.verified === undefined
      ? []
      : Array.isArray(metadata.verified)
        ? metadata.verified
        : [metadata.verified]
  verified.forEach((event, index) =>
    validateActorEvent(file, event, `verified[${index}]`, { requireAt: true }),
  )

  const sourceIds = new Set()
  if (metadata.sources !== undefined && !Array.isArray(metadata.sources)) {
    fail(file, 'sources must be a list')
  }
  for (const [index, source] of (metadata.sources ?? []).entries()) {
    if (source === null || typeof source !== 'object' || Array.isArray(source)) {
      fail(file, `sources[${index}] must be a mapping`)
      continue
    }
    validatePath(file, source.resource, `sources[${index}].resource`)
    if (source.id !== undefined) {
      if (typeof source.id !== 'string' || source.id.length === 0) {
        fail(file, `sources[${index}].id must be a non-empty string`)
      } else if (sourceIds.has(source.id)) {
        fail(file, `duplicate source id: ${source.id}`)
      } else {
        sourceIds.add(source.id)
      }
    }
    if (source.last_modified !== undefined && !hasExplicitOffset(source.last_modified)) {
      fail(file, `sources[${index}].last_modified must have an explicit UTC offset`)
    }
  }

  const definitions = new Set([...text.matchAll(/^\[\^([^\]]+)\]:/gm)].map((match) => match[1]))
  const bodyWithoutDefinitions = text.replace(/^\[\^[^\]]+\]:.*$/gm, '')
  for (const match of bodyWithoutDefinitions.matchAll(/\[\^([^\]]+)\]/g)) {
    const id = match[1]
    if (!sourceIds.has(id)) fail(file, `footnote ${id} has no matching sources[].id`)
    if (!definitions.has(id)) fail(file, `footnote ${id} has no definition`)
  }
  for (const id of definitions) {
    if (!sourceIds.has(id)) fail(file, `footnote definition ${id} has no matching sources[].id`)
  }
}

function validateLinks(file, text) {
  for (const match of text.matchAll(/\[[^\]]+\]\(([^)]+)\)/g)) {
    const href = match[1]
    if (/^(?:https?:|mailto:|#)/i.test(href)) continue
    const clean = href.split('#', 1)[0].split('?', 1)[0]
    if (clean.length === 0) continue
    const target = resolveReference(file, clean)
    if (!existsSync(target) && !existsSync(`${target}.md`))
      fail(file, `broken Markdown link: ${href}`)
  }
}

if (!existsSync(bundleRoot)) {
  console.error(`OKF bundle not found: ${bundleRoot}`)
  process.exit(1)
}

const files = markdownFiles(bundleRoot)
for (const file of files) {
  const text = readFileSync(file, 'utf8')
  const metadata = frontmatter(text, file)
  const basename = path.basename(file)
  const relative = path.relative(bundleRoot, file)

  if (basename === 'index.md') {
    counts.indexes += 1
    if (relative === 'index.md') {
      if (metadata?.okf_version !== '0.2') fail(file, 'bundle root must declare okf_version: "0.2"')
    } else if (metadata !== undefined) {
      fail(file, 'nested index.md must not have frontmatter')
    }
  } else if (basename === 'log.md') {
    counts.logs += 1
    if (metadata !== undefined) fail(file, 'log.md must not have frontmatter')
    for (const match of text.matchAll(/^##\s+(.+)$/gm)) {
      if (!/^\d{4}-\d{2}-\d{2}$/.test(match[1])) fail(file, `invalid log date heading: ${match[1]}`)
    }
  } else {
    validateConcept(file, text, metadata)
  }

  validateLinks(file, text)
}

if (errors.length > 0) {
  console.error(errors.join('\n'))
  console.error(
    `OKF validation failed with ${errors.length} error${errors.length === 1 ? '' : 's'}.`,
  )
  process.exit(1)
}

console.log(
  `OKF validation passed: ${files.length} files, ${counts.concepts} concepts, ${counts.indexes} indexes, ${counts.logs} log.`,
)
