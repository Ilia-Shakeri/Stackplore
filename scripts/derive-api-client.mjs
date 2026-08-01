import { createClient } from '@hey-api/openapi-ts';
import { copyFile, mkdir, mkdtemp, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, relative, resolve, sep } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { format } from 'prettier';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const contractPath = join(repositoryRoot, 'contracts', 'openapi.yaml');
const committedOutput = join(repositoryRoot, 'packages', 'api-client', 'src', 'derived');
const packageTypeScriptConfig = join(repositoryRoot, 'packages', 'api-client', 'tsconfig.json');

const collectFiles = async (directory, base = directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];

  for (const entry of entries.sort((left, right) => left.name.localeCompare(right.name))) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectFiles(path, base)));
    } else if (entry.isFile()) {
      files.push(relative(base, path).split(sep).join('/'));
    }
  }

  return files;
};

const derive = async (outputPath) => {
  await createClient({
    input: contractPath,
    interactive: false,
    logs: {
      file: false,
      level: 'silent',
    },
    output: {
      clean: true,
      entryFile: false,
      fileName: {
        suffix: '.derived',
      },
      header: [],
      path: outputPath,
      tsConfigPath: packageTypeScriptConfig,
    },
    plugins: [
      {
        name: '@hey-api/typescript',
        enums: false,
      },
    ],
  });

  const files = await collectFiles(outputPath);
  for (const file of files) {
    if (!file.endsWith('.ts')) {
      continue;
    }
    const path = join(outputPath, file);
    const source = await readFile(path, 'utf8');
    const formatted = await format(source, {
      parser: 'typescript',
      printWidth: 100,
      semi: true,
      singleQuote: true,
      trailingComma: 'all',
    });
    await writeFile(path, formatted, 'utf8');
  }
};

const copyDerivedFiles = async (source, target, files) => {
  const expectedTarget = resolve(repositoryRoot, 'packages', 'api-client', 'src', 'derived');
  if (resolve(target) !== expectedTarget) {
    throw new Error('Refusing to replace an unexpected derived directory.');
  }

  await rm(target, { force: true, recursive: true });
  for (const file of files) {
    const destination = join(target, file);
    await mkdir(dirname(destination), { recursive: true });
    await copyFile(join(source, file), destination);
  }
};

const compareDerivedFiles = async (expectedDirectory, actualDirectory, files) => {
  let committedFiles;
  try {
    committedFiles = await collectFiles(actualDirectory);
  } catch (error) {
    if (error instanceof Error && 'code' in error && error.code === 'ENOENT') {
      throw new Error('Derived client types are missing. Run the write command.', { cause: error });
    }
    throw error;
  }

  if (committedFiles.join('\n') !== files.join('\n')) {
    throw new Error('Derived client file set is stale. Run the write command.');
  }

  for (const file of files) {
    const expected = await readFile(join(expectedDirectory, file));
    const actual = await readFile(join(actualDirectory, file));
    if (!expected.equals(actual)) {
      throw new Error(`Derived client file is stale: ${file}. Run the write command.`);
    }
  }
};

const main = async () => {
  const mode = process.argv[2] ?? '--check';
  if (mode !== '--check' && mode !== '--write') {
    throw new Error('Usage: node scripts/derive-api-client.mjs --check|--write');
  }

  const temporaryDirectory = await mkdtemp(join(tmpdir(), 'stackplore-api-client-derive-'));
  const temporaryOutput = join(temporaryDirectory, 'derived');

  try {
    await derive(temporaryOutput);
    const files = await collectFiles(temporaryOutput);
    if (files.length !== 1 || files[0] !== 'types.derived.ts') {
      throw new Error(`Unexpected derived file set: ${files.join(', ') || 'empty'}.`);
    }

    const derivedSource = await readFile(join(temporaryOutput, files[0]), 'utf8');
    if (derivedSource.includes('\r') || !derivedSource.endsWith('\n')) {
      throw new Error('Derived client must use LF and end with a newline.');
    }

    if (mode === '--write') {
      await copyDerivedFiles(temporaryOutput, committedOutput, files);
      process.stdout.write('Derived client types updated.\n');
      return;
    }

    await compareDerivedFiles(temporaryOutput, committedOutput, files);
    process.stdout.write('Derived client types match the OpenAPI contract.\n');
  } finally {
    await rm(temporaryDirectory, { force: true, recursive: true });
  }
};

try {
  await main();
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`${message}\n`);
  process.exitCode = 1;
}
