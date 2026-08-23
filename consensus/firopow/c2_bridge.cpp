#include <crypto/progpow/include/ethash/ethash.hpp>
#include <crypto/progpow/include/ethash/progpow.hpp>

#include <cstdint>
#include <cstring>
#include <mutex>

namespace {

// Candidate-only, deliberately conservative one-epoch cache. This is not a
// production cache policy or a mining implementation.
std::mutex epoch_mutex;
int cached_epoch = -1;
ethash::epoch_context_ptr cached_context{nullptr, ethash_destroy_epoch_context};

const ethash::epoch_context* get_epoch_context(int block_number) {
    const auto epoch = ethash::get_epoch_number(block_number);
    if (!cached_context || cached_epoch != epoch) {
        cached_context = ethash::create_epoch_context(epoch);
        if (!cached_context) return nullptr;
        cached_epoch = epoch;
    }
    return cached_context.get();
}

}  // namespace

extern "C" int aichain_firopow_hash(
    int block_number,
    const uint8_t header_hash[32],
    uint64_t nonce,
    uint8_t mix_hash_out[32],
    uint8_t final_hash_out[32]) {
    std::lock_guard<std::mutex> lock{epoch_mutex};
    const auto* context = get_epoch_context(block_number);
    if (!context) return 0;
    const auto result = progpow::hash(*context, block_number, ethash::hash256_from_bytes(header_hash), nonce);
    std::memcpy(mix_hash_out, result.mix_hash.bytes, 32);
    std::memcpy(final_hash_out, result.final_hash.bytes, 32);
    return 1;
}

extern "C" int aichain_firopow_verify(
    int block_number,
    const uint8_t header_hash[32],
    const uint8_t mix_hash[32],
    uint64_t nonce,
    const uint8_t boundary[32]) {
    std::lock_guard<std::mutex> lock{epoch_mutex};
    const auto* context = get_epoch_context(block_number);
    if (!context) return 0;
    return progpow::verify(
               *context,
               block_number,
               ethash::hash256_from_bytes(header_hash),
               ethash::hash256_from_bytes(mix_hash),
               nonce,
               ethash::hash256_from_bytes(boundary))
        ? 1
        : 0;
}
