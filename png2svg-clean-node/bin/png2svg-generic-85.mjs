#!/usr/bin/env node

import { execFile } from 'node:child_process';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  ColorMode,
  Hierarchical,
  OptimizePreset,
  PathSimplifyMode,
  optimize,
  vectorize
} from '@neplex/vectorizer';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const packageDir = resolve(__dirname, '..');

const enumMaps = {
  colorMode: ColorMode,
  hierarchical: Hierarchical,
  mode: PathSimplifyMode
};

const optimizePresetMap = {
  Safe: OptimizePreset.Safe
};

function parseArgs(argv) {
  const options = {
    profile: 'generic-85',
    profileDir: join(packageDir, 'profiles'),
    preview: '',
    optimizedPng: '',
    keepTemp: false
  };
  const positional = [];

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];

    if (arg === '--profile') {
      options.profile = argv[++index];
    } else if (arg === '--profile-dir') {
      options.profileDir = argv[++index];
    } else if (arg === '--preview') {
      options.preview = argv[++index];
    } else if (arg === '--optimized-png') {
      options.optimizedPng = argv[++index];
    } else if (arg === '--keep-temp') {
      options.keepTemp = true;
    } else {
      positional.push(arg);
    }
  }

  if (positional.length !== 2 || !options.profile) {
    throw new Error(
      'Usage: node bin/png2svg-generic-85.mjs input.png output.svg [--optimized-png output.png] [--preview output.png]'
    );
  }

  return {
    inputPath: positional[0],
    outputPath: positional[1],
    ...options
  };
}

async function loadProfile(options) {
  const profilePath = extname(options.profile)
    ? options.profile
    : join(options.profileDir, `${options.profile}.json`);
  const profile = JSON.parse(await readFile(profilePath, 'utf8'));
  return { profile, profilePath };
}

function normalizeVectorizerConfig(config) {
  const normalized = { ...config };

  for (const [key, enumMap] of Object.entries(enumMaps)) {
    if (typeof normalized[key] === 'string') {
      const value = enumMap[normalized[key]];
      if (value === undefined) {
        throw new Error(`Invalid vectorizer enum ${key}: ${normalized[key]}`);
      }
      normalized[key] = value;
    }
  }

  return normalized;
}

function normalizeOptimizeConfig(config = {}) {
  const normalized = { ...config };
  if (typeof normalized.preset === 'string') {
    const preset = optimizePresetMap[normalized.preset];
    if (preset === undefined) {
      throw new Error(`Invalid optimize preset: ${normalized.preset}`);
    }
    normalized.preset = preset;
  }
  return normalized;
}

function runCommand(command, args) {
  return new Promise((resolvePromise, reject) => {
    const child = execFile(command, args);
    const stderr = [];

    child.stderr.on('data', (chunk) => stderr.push(chunk));
    child.on('error', reject);
    child.on('close', (code) => {
      if (code === 0) {
        resolvePromise();
        return;
      }

      reject(new Error(`${command} exited with ${code}: ${Buffer.concat(stderr).toString('utf8')}`));
    });
  });
}

async function ensureParentDir(filePath) {
  await mkdir(dirname(resolve(filePath)), { recursive: true });
}

function buildMagickPreprocessArgs(inputPath, outputPath, profile) {
  const preprocess = profile.preprocess ?? {};
  const args = [inputPath];

  if (preprocess.alphaOff) args.push('-alpha', 'off');
  if (preprocess.fuzz) args.push('-fuzz', preprocess.fuzz);
  if (preprocess.fill) args.push('-fill', preprocess.fill);
  if (preprocess.opaque) args.push('-opaque', preprocess.opaque);
  if (preprocess.colors) args.push('-colors', String(preprocess.colors));
  if (preprocess.dither) args.push('-dither', preprocess.dither);

  args.push(outputPath);
  return args;
}

async function vectorizeSvg(inputPath, outputPath, profile) {
  const image = await readFile(inputPath);
  const rawSvg = await vectorize(image, normalizeVectorizerConfig(profile.vectorizer));
  const optimizedSvg = await optimize(rawSvg, normalizeOptimizeConfig(profile.optimize));
  await ensureParentDir(outputPath);
  await writeFile(outputPath, optimizedSvg, 'utf8');
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  const { profile, profilePath } = await loadProfile(options);
  const tempDir = await mkdtemp(join(tmpdir(), 'png2svg-generic-85-'));
  const optimizedPath = join(tempDir, 'source-optimized.png');

  try {
    await runCommand('magick', buildMagickPreprocessArgs(options.inputPath, optimizedPath, profile));
    await vectorizeSvg(optimizedPath, options.outputPath, profile);

    if (options.optimizedPng) {
      await ensureParentDir(options.optimizedPng);
      await runCommand('magick', [optimizedPath, options.optimizedPng]);
    }

    if (options.preview) {
      await ensureParentDir(options.preview);
      await runCommand('magick', ['-background', 'white', options.outputPath, options.preview]);
    }

    console.log(JSON.stringify({
      profile: profile.name,
      profilePath,
      input: options.inputPath,
      output: options.outputPath,
      optimizedPng: options.optimizedPng || null,
      preview: options.preview || null
    }));
  } finally {
    if (!options.keepTemp) {
      await rm(tempDir, { recursive: true, force: true });
    }
  }
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
