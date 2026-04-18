<script setup lang="ts">
import { reactive, watch } from 'vue';
import type { PHPRuntimeRecord } from '@contracts';
import type { SiteFormErrors, SiteFormModel } from './site-form.types';

interface SiteFormProps {
  modelValue: SiteFormModel;
  runtimeOptions: PHPRuntimeRecord[];
  title: string;
  description: string;
  submitLabel: string;
  busy?: boolean;
  errors?: SiteFormErrors;
}

const props = withDefaults(defineProps<SiteFormProps>(), {
  busy: false,
  errors: () => ({}),
});

const emit = defineEmits<{
  (event: 'update:modelValue', value: SiteFormModel): void;
  (event: 'submit'): void;
}>();

const localModel = reactive<SiteFormModel>({
  rootPath: props.modelValue.rootPath,
  domain: props.modelValue.domain,
  phpVersion: props.modelValue.phpVersion,
});

watch(
  () => props.modelValue,
  (value) => {
    localModel.rootPath = value.rootPath;
    localModel.domain = value.domain;
    localModel.phpVersion = value.phpVersion;
  },
  { deep: true }
);

watch(
  localModel,
  (value) => {
    emit('update:modelValue', { ...value });
  },
  { deep: true }
);
</script>

<template>
  <section class="panel form-stack">
    <header>
      <h2 class="section-title">{{ title }}</h2>
      <p class="page-copy">{{ description }}</p>
    </header>

    <p v-if="props.errors.general" class="message message--error">
      {{ props.errors.general }}
    </p>

    <div class="field">
      <label for="site-root-path">Laravel root path</label>
      <input
        id="site-root-path"
        v-model="localModel.rootPath"
        autocomplete="off"
        placeholder="/home/you/projects/my-app"
      />
      <span v-if="props.errors.rootPath" class="field-error">{{ props.errors.rootPath }}</span>
    </div>

    <div class="field">
      <label for="site-domain">.test domain</label>
      <input
        id="site-domain"
        v-model="localModel.domain"
        autocomplete="off"
        placeholder="my-app.test"
      />
      <span v-if="props.errors.domain" class="field-error">{{ props.errors.domain }}</span>
    </div>

    <div class="field">
      <label for="site-runtime">PHP runtime</label>
      <select id="site-runtime" v-model="localModel.phpVersion">
        <option value="">Use the default runtime</option>
        <option
          v-for="runtime in props.runtimeOptions"
          :key="runtime.version"
          :value="runtime.version"
        >
          PHP {{ runtime.version }} · {{ runtime.fpmService }}
        </option>
      </select>
      <span v-if="props.errors.phpVersion" class="field-error">{{ props.errors.phpVersion }}</span>
    </div>

    <button class="button" :disabled="props.busy" type="button" @click="emit('submit')">
      {{ props.busy ? 'Working…' : props.submitLabel }}
    </button>
  </section>
</template>
