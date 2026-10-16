// This source shim keeps the runtime dependency explicit. Tachyon's atomic
// post-bundle hook publishes a self-contained browser ESM build at this path.
export {
  AutoModelForSeq2SeqLM,
  AutoTokenizer,
  env,
  Tensor,
} from '@huggingface/transformers'
