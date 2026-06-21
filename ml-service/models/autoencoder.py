import numpy as np
import torch
import torch.nn as nn
from torch.utils.data import DataLoader, TensorDataset


class _EncoderDecoder(nn.Module):
    def __init__(self, num_features: int, sequence_length: int, latent_dim: int = 32):
        super().__init__()
        self.sequence_length = sequence_length
        self.num_features = num_features

        self.encoder_lstm1 = nn.LSTM(num_features, 64, batch_first=True)
        self.encoder_lstm2 = nn.LSTM(64, latent_dim, batch_first=True)

        self.decoder_lstm1 = nn.LSTM(latent_dim, 32, batch_first=True)
        self.decoder_lstm2 = nn.LSTM(32, 64, batch_first=True)
        self.output_layer = nn.Linear(64, num_features)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        encoded, _ = self.encoder_lstm1(x)
        encoded, (hidden, _) = self.encoder_lstm2(encoded)
        latent = hidden[-1]

        repeated = latent.unsqueeze(1).repeat(1, self.sequence_length, 1)
        decoded, _ = self.decoder_lstm1(repeated)
        decoded, _ = self.decoder_lstm2(decoded)
        reconstructed = self.output_layer(decoded)
        return reconstructed


class LSTMAutoencoder:
    def __init__(self, num_features: int = 18, sequence_length: int = 60):
        self.num_features = num_features
        self.sequence_length = sequence_length
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.model = _EncoderDecoder(num_features, sequence_length).to(self.device)
        self.threshold_mean: float = 0.0
        self.threshold_std: float = 1.0

    def train(
        self,
        sequences: np.ndarray,
        epochs: int = 50,
        batch_size: int = 32,
        learning_rate: float = 1e-3,
    ) -> dict:
        self.model.train()
        tensor_data = torch.FloatTensor(sequences).to(self.device)
        dataset = TensorDataset(tensor_data, tensor_data)
        loader = DataLoader(dataset, batch_size=batch_size, shuffle=True)

        optimizer = torch.optim.Adam(self.model.parameters(), lr=learning_rate)
        criterion = nn.MSELoss()
        train_losses = []

        for epoch in range(epochs):
            epoch_loss = 0.0
            batch_count = 0
            for batch_input, batch_target in loader:
                optimizer.zero_grad()
                reconstructed = self.model(batch_input)
                loss = criterion(reconstructed, batch_target)
                loss.backward()
                optimizer.step()
                epoch_loss += loss.item()
                batch_count += 1
            train_losses.append(epoch_loss / batch_count)

        return {"train_losses": train_losses}

    def anomaly_score(self, sequence: np.ndarray) -> float:
        self.model.eval()
        with torch.no_grad():
            tensor_input = torch.FloatTensor(sequence).unsqueeze(0).to(self.device)
            reconstructed = self.model(tensor_input)
            mse = nn.MSELoss()(reconstructed, tensor_input)
            return float(mse.item())

    def per_feature_error(self, sequence: np.ndarray) -> list[float]:
        self.model.eval()
        with torch.no_grad():
            tensor_input = torch.FloatTensor(sequence).unsqueeze(0).to(self.device)
            reconstructed = self.model(tensor_input)
            per_feature_mse = ((reconstructed - tensor_input) ** 2).mean(dim=1).squeeze(0)
            return per_feature_mse.cpu().tolist()

    def calibrate_threshold(self, clean_sequences: np.ndarray) -> dict:
        scores = [self.anomaly_score(seq) for seq in clean_sequences]
        self.threshold_mean = float(np.mean(scores))
        self.threshold_std = float(np.std(scores))
        return {"mean": self.threshold_mean, "std": self.threshold_std}

    def is_anomalous(self, score: float, std_multiplier: float = 3.0) -> bool:
        return score > self.threshold_mean + std_multiplier * self.threshold_std

    def save(self, path: str):
        torch.save(
            {
                "model_state": self.model.state_dict(),
                "threshold_mean": self.threshold_mean,
                "threshold_std": self.threshold_std,
                "num_features": self.num_features,
                "sequence_length": self.sequence_length,
            },
            path,
        )

    @classmethod
    def load(cls, path: str, num_features: int = 18, sequence_length: int = 60) -> "LSTMAutoencoder":
        autoencoder = cls(num_features, sequence_length)
        checkpoint = torch.load(path, map_location=autoencoder.device, weights_only=True)
        autoencoder.model.load_state_dict(checkpoint["model_state"])
        autoencoder.threshold_mean = checkpoint["threshold_mean"]
        autoencoder.threshold_std = checkpoint["threshold_std"]
        return autoencoder
