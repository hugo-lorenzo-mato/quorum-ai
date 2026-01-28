import { useCallback } from 'react';
import { Mic, MicOff } from 'lucide-react';
import { useSpeechRecognition } from '../hooks/useSpeechRecognition';

/**
 * Voice input button component that uses Web Speech API
 * @param {Object} props
 * @param {Function} props.onTranscript - Callback when speech is transcribed
 * @param {boolean} props.disabled - Whether the button is disabled
 * @param {string} props.className - Additional CSS classes
 * @param {string} props.lang - Language code (default: 'es-ES')
 */
export function VoiceInputButton({
  onTranscript,
  disabled = false,
  className = '',
  lang = 'es-ES',
}) {
  const handleResult = useCallback(
    (transcript) => {
      if (onTranscript && transcript) {
        onTranscript(transcript);
      }
    },
    [onTranscript]
  );

  const handleError = useCallback((errorMessage) => {
    console.warn('Speech recognition error:', errorMessage);
  }, []);

  const {
    isListening,
    isSupported,
    toggleListening,
  } = useSpeechRecognition({
    onResult: handleResult,
    onError: handleError,
    lang,
  });

  // Don't render if not supported
  if (!isSupported) {
    return (
      <button
        type="button"
        disabled
        className={`p-1.5 rounded-lg text-muted-foreground opacity-50 cursor-not-allowed ${className}`}
        title="Tu navegador no soporta entrada por voz"
      >
        <MicOff className="w-5 h-5" />
      </button>
    );
  }

  const handleClick = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (!disabled) {
      toggleListening();
    }
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={disabled}
      className={`
        p-1.5 rounded-lg transition-all
        focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background
        ${isListening
          ? 'text-red-500 bg-red-500/10 animate-pulse'
          : 'text-muted-foreground hover:text-foreground hover:bg-accent'
        }
        ${disabled ? 'opacity-50 cursor-not-allowed' : ''}
        ${className}
      `}
      title={isListening ? 'Detener grabación' : 'Iniciar grabación por voz'}
      aria-label={isListening ? 'Detener grabación' : 'Iniciar grabación por voz'}
      aria-pressed={isListening}
    >
      <Mic className="w-5 h-5" />
    </button>
  );
}

export default VoiceInputButton;
