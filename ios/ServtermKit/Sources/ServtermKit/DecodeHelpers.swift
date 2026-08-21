import Foundation

extension KeyedDecodingContainer {
    /// or reads one key and falls back to a safe default when the daemon
    /// does not send it. The app must keep working when the daemon adds or
    /// removes a field.
    func or<T: Decodable>(_ key: Key, _ fallback: T) -> T {
        ((try? decodeIfPresent(T.self, forKey: key)) ?? nil) ?? fallback
    }

    /// maybe reads one key that has no default. It stays nil when the key
    /// is absent or null, and a nil value always means "no reading".
    func maybe<T: Decodable>(_ key: Key) -> T? {
        (try? decodeIfPresent(T.self, forKey: key)) ?? nil
    }
}
